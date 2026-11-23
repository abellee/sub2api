package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

const DefaultInflationPrompt = "请用中文写一段约300字的短文，介绍雨天散步的感受。全文仅用中文，不夹杂英文。直接输出正文，不要思考过程，也不要调用工具。"

type InflationMonitorPlan struct {
	GroupID         int64   `json:"group_id"`
	Enabled         bool    `json:"enabled"`
	IntervalMinutes int     `json:"interval_minutes"`
	AccountIDs      []int64 `json:"account_ids"`
	Model           string  `json:"model"`
	Prompt          string  `json:"prompt"`
	Encoding        string  `json:"encoding"`
	ReasoningEffort string  `json:"reasoning_effort"`
	Aggregation     string  `json:"aggregation"`
	YellowThreshold float64 `json:"yellow_threshold"`
	RedThreshold    float64 `json:"red_threshold"`
}
type InflationMonitorSample struct {
	AccountID       int64    `json:"account_id,omitempty"`
	Model           string   `json:"model"`
	InputTokens     *int     `json:"input_tokens"`
	OutputTokens    *int     `json:"output_tokens"`
	TextTokens      int      `json:"text_tokens"`
	ReasoningTokens *int     `json:"reasoning_tokens"`
	Rate            *float64 `json:"rate"`
	Error           string   `json:"error,omitempty"`
}
type InflationMonitorResult struct {
	MeasuredAt      time.Time                `json:"measured_at"`
	Prompt          string                   `json:"prompt"`
	Encoding        string                   `json:"encoding"`
	ReasoningEffort string                   `json:"reasoning_effort"`
	Samples         []InflationMonitorSample `json:"samples"`
}
type InflationMonitorHistory struct {
	GroupID         int64                    `json:"group_id"`
	Aggregation     string                   `json:"aggregation"`
	YellowThreshold float64                  `json:"yellow_threshold"`
	RedThreshold    float64                  `json:"red_threshold"`
	Results         []InflationMonitorResult `json:"results"`
}
type InflationMonitorSummary struct {
	GroupID         int64      `json:"group_id"`
	Rate            *float64   `json:"rate"`
	Aggregation     string     `json:"aggregation"`
	YellowThreshold float64    `json:"yellow_threshold"`
	RedThreshold    float64    `json:"red_threshold"`
	MeasuredAt      *time.Time `json:"measured_at"`
}

type InflationMonitorStatus struct {
	InflationMonitorSummary
	Running bool `json:"running"`
}

type inflationMonitorProbe func(context.Context, int64, InflationMonitorPlan) InflationMonitorSample

type InflationMonitorService struct {
	db         *sql.DB
	tester     *AccountTestService
	probe      inflationMonitorProbe
	cancel     context.CancelFunc
	done       chan struct{}
	once       sync.Once
	workers    chan struct{}
	jobs       sync.WaitGroup
	runCtx     context.Context
	dispatchMu sync.Mutex
	stopping   bool
}

func NewInflationMonitorService(db *sql.DB, tester *AccountTestService) *InflationMonitorService {
	ctx, cancel := context.WithCancel(context.Background())
	s := &InflationMonitorService{runCtx: ctx, cancel: cancel, db: db, tester: tester, done: make(chan struct{}), workers: make(chan struct{}, 4)}
	s.probe = s.probeAccount
	return s
}
func (s *InflationMonitorService) Start() {
	s.once.Do(func() {
		ctx := s.runCtx
		go func() {
			defer close(s.done)
			defer s.jobs.Wait()
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					s.tick(ctx)
				}
			}
		}()
	})
}
func (s *InflationMonitorService) Stop() {
	if s != nil && s.cancel != nil {
		s.dispatchMu.Lock()
		s.stopping = true
		s.cancel()
		s.dispatchMu.Unlock()
		<-s.done
	}
}
func (s *InflationMonitorService) Plans(ctx context.Context) ([]InflationMonitorPlan, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT config FROM inflation_monitor_plans ORDER BY group_id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	plans := []InflationMonitorPlan{}
	for rows.Next() {
		var raw []byte
		var p InflationMonitorPlan
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		plans = append(plans, p)
	}
	return plans, rows.Err()
}
func validateInflationPlan(p InflationMonitorPlan) error {
	if p.GroupID <= 0 || p.IntervalMinutes < 1 || p.IntervalMinutes > 1440 {
		return fmt.Errorf("分组无效，检测间隔须为 1–1440 分钟")
	}
	if len(p.AccountIDs) > 20 || (p.Enabled && len(p.AccountIDs) == 0) {
		return fmt.Errorf("启用时请选择 1–20 个账号")
	}
	if strings.TrimSpace(p.Model) == "" || len(p.Model) > 200 || strings.TrimSpace(p.Prompt) == "" || len(p.Prompt) > 32000 {
		return fmt.Errorf("请填写文本模型和提示词（最多 32000 字节）")
	}
	if p.Encoding != "o200k_base" && p.Encoding != "cl100k_base" {
		return fmt.Errorf("不支持的分词器")
	}
	if p.Aggregation != "mean" && p.Aggregation != "median" {
		return fmt.Errorf("请选择平均值或中位数")
	}
	if math.IsNaN(p.YellowThreshold) || math.IsInf(p.YellowThreshold, 0) || math.IsNaN(p.RedThreshold) || math.IsInf(p.RedThreshold, 0) || p.YellowThreshold < 0 || p.RedThreshold <= p.YellowThreshold {
		return fmt.Errorf("黄灯阈值须不小于 0，红灯阈值须大于黄灯阈值")
	}
	switch p.ReasoningEffort {
	case "", "none", "minimal", "low", "medium", "high", "xhigh":
	default:
		return fmt.Errorf("不支持的思考等级")
	}
	seen := map[int64]bool{}
	for _, id := range p.AccountIDs {
		if id <= 0 || seen[id] {
			return fmt.Errorf("账号无效或重复")
		}
		seen[id] = true
	}
	return nil
}
func (s *InflationMonitorService) SavePlans(ctx context.Context, plans []InflationMonitorPlan) error {
	if len(plans) > 100 {
		return fmt.Errorf("最多配置 100 个分组")
	}
	seen := map[int64]bool{}
	for _, p := range plans {
		if err := validateInflationPlan(p); err != nil {
			return err
		}
		if seen[p.GroupID] {
			return fmt.Errorf("分组重复")
		}
		seen[p.GroupID] = true
		var exists bool
		if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM groups WHERE id=$1 AND deleted_at IS NULL)`, p.GroupID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("分组不存在")
		}
		for _, id := range p.AccountIDs {
			account, err := s.tester.accountRepo.GetByID(ctx, id)
			if err != nil {
				return fmt.Errorf("账号 %d 不存在", id)
			}
			belongs := false
			for _, gid := range account.GroupIDs {
				if gid == p.GroupID {
					belongs = true
				}
			}
			if !belongs {
				return fmt.Errorf("账号 %d 不属于所选分组", id)
			}
			mapped := strings.ToLower(account.GetMappedModel(p.Model))
			if strings.Contains(mapped, "image") || strings.Contains(mapped, "video") || strings.Contains(mapped, "audio") || strings.Contains(mapped, "embedding") || account.IsSyntheticUITest() || account.IsTypeSafe() {
				return fmt.Errorf("仅支持真实账号的文本模型")
			}
			if p.ReasoningEffort != "" && !account.IsOpenAI() {
				return fmt.Errorf("非 OpenAI 账号请选择默认思考等级")
			}
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	ids := []int64{}
	for _, p := range plans {
		raw, err := json.Marshal(p)
		if err != nil {
			return err
		}
		ids = append(ids, p.GroupID)
		_, err = tx.ExecContext(ctx, `INSERT INTO inflation_monitor_plans(group_id,config,revision,next_run_at) VALUES($1,$2,$3,NOW()+($4*INTERVAL '1 minute')) ON CONFLICT(group_id) DO UPDATE SET config=EXCLUDED.config,revision=EXCLUDED.revision,next_run_at=EXCLUDED.next_run_at,updated_at=NOW(),lease_until=NULL,lease_token=NULL WHERE inflation_monitor_plans.config IS DISTINCT FROM EXCLUDED.config`, p.GroupID, string(raw), uuid.NewString(), p.IntervalMinutes)
		if err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM inflation_monitor_plans WHERE NOT(group_id=ANY($1::bigint[]))`, pq.Array(ids)); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *InflationMonitorService) tick(ctx context.Context) {
	plans, err := s.Plans(ctx)
	if err != nil {
		slog.Error("inflation monitor plans", "error", err)
		return
	}
	for _, p := range plans {
		if ctx.Err() != nil {
			return
		}
		if !p.Enabled {
			continue
		}
		if _, err := s.startGroup(ctx, p, false); err != nil {
			slog.Error("inflation monitor claim", "error", err)
		}
	}
}

// Manual and scheduled runs share one database lease, including across replicas.
func (s *InflationMonitorService) startGroup(ctx context.Context, p InflationMonitorPlan, force bool) (bool, error) {
	s.dispatchMu.Lock()
	defer s.dispatchMu.Unlock()
	if s.stopping {
		return false, infraerrors.ServiceUnavailable("INFLATION_MONITOR_STOPPING", "检测服务正在停止")
	}
	select {
	case s.workers <- struct{}{}:
	default:
		return false, nil
	}
	token := uuid.NewString()
	var revision string
	err := s.db.QueryRowContext(ctx, `UPDATE inflation_monitor_plans SET lease_token=$2,lease_until=NOW()+($3*INTERVAL '1 second'),next_run_at=NOW()+($4*INTERVAL '1 minute') WHERE group_id=$1 AND config=$5::jsonb AND ($6 OR next_run_at<=NOW()) AND (lease_until IS NULL OR lease_until<NOW()) RETURNING revision`, p.GroupID, token, len(p.AccountIDs)*125+60, p.IntervalMinutes, mustInflationJSON(p), force).Scan(&revision)
	if err != nil {
		<-s.workers
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	s.jobs.Add(1)
	go func() {
		defer s.jobs.Done()
		defer func() { <-s.workers }()
		s.runGroup(s.runCtx, p, token, revision)
	}()
	return true, nil
}

func (s *InflationMonitorService) RunNow(ctx context.Context, groupID int64) error {
	var raw []byte
	err := s.db.QueryRowContext(ctx, `SELECT config FROM inflation_monitor_plans WHERE group_id=$1`, groupID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return infraerrors.NotFound("INFLATION_MONITOR_NOT_FOUND", "请先保存该分组的检测配置")
	}
	if err != nil {
		return err
	}
	var p InflationMonitorPlan
	if err = json.Unmarshal(raw, &p); err != nil {
		return err
	}
	if err = validateInflationPlan(p); err != nil {
		return infraerrors.BadRequest("INFLATION_MONITOR_INVALID", err.Error())
	}
	if len(p.AccountIDs) == 0 {
		return infraerrors.BadRequest("INFLATION_MONITOR_NO_ACCOUNTS", "请先选择参与检测的账号")
	}
	started, err := s.startGroup(ctx, p, true)
	if err != nil {
		return err
	}
	if !started {
		return infraerrors.Conflict("INFLATION_MONITOR_BUSY", "该分组正在检测或检测任务已满，请稍后再试")
	}
	return nil
}

func (s *InflationMonitorService) Statuses(ctx context.Context) ([]InflationMonitorStatus, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT p.group_id,p.config,r.result,COALESCE(p.lease_until>NOW(),false) FROM inflation_monitor_plans p LEFT JOIN LATERAL(SELECT result FROM inflation_monitor_results WHERE group_id=p.group_id ORDER BY id DESC LIMIT 1)r ON true ORDER BY p.group_id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := []InflationMonitorStatus{}
	for rows.Next() {
		var id int64
		var config, raw []byte
		var running bool
		if err = rows.Scan(&id, &config, &raw, &running); err != nil {
			return nil, err
		}
		var p InflationMonitorPlan
		if err = json.Unmarshal(config, &p); err != nil {
			return nil, err
		}
		item := InflationMonitorStatus{InflationMonitorSummary: InflationMonitorSummary{GroupID: id, Aggregation: p.Aggregation, YellowThreshold: p.YellowThreshold, RedThreshold: p.RedThreshold}, Running: running}
		if len(raw) > 0 {
			var result InflationMonitorResult
			if err = json.Unmarshal(raw, &result); err != nil {
				return nil, err
			}
			item.Rate = InflationAggregate(result.Samples, p.Aggregation)
			item.MeasuredAt = &result.MeasuredAt
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *InflationMonitorService) runGroup(ctx context.Context, p InflationMonitorPlan, token, revision string) {
	defer func() {
		if ctx.Err() != nil {
			releaseCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if _, err := s.db.ExecContext(releaseCtx, `UPDATE inflation_monitor_plans SET lease_until=NULL,lease_token=NULL WHERE group_id=$1 AND lease_token=$2`, p.GroupID, token); err != nil {
				slog.Error("inflation monitor release", "error", err)
			}
		}
	}()
	result := InflationMonitorResult{MeasuredAt: time.Now(), Prompt: p.Prompt, Encoding: p.Encoding, ReasoningEffort: p.ReasoningEffort, Samples: []InflationMonitorSample{}}
	for _, id := range p.AccountIDs {
		if ctx.Err() != nil {
			return
		}
		var current bool
		if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM inflation_monitor_plans WHERE group_id=$1 AND revision=$2 AND lease_token=$3)`, p.GroupID, revision, token).Scan(&current); err != nil || !current {
			break
		}
		result.Samples = append(result.Samples, s.probe(ctx, id, p))
	}
	result.MeasuredAt = time.Now()
	if err := s.saveResult(ctx, p.GroupID, token, revision, result); err != nil {
		slog.Error("inflation monitor save", "error", err)
	}
}

func mustInflationJSON(value any) string { raw, _ := json.Marshal(value); return string(raw) }
func (s *InflationMonitorService) saveResult(ctx context.Context, groupID int64, token, revision string, result InflationMonitorResult) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var current bool
	err = tx.QueryRowContext(ctx, `SELECT revision=$3 AND lease_token=$2 FROM inflation_monitor_plans WHERE group_id=$1 FOR UPDATE`, groupID, token, revision).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if !current {
		return nil
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO inflation_monitor_results(group_id,measured_at,result) VALUES($1,$2,$3)`, groupID, result.MeasuredAt, mustInflationJSON(result)); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM inflation_monitor_results WHERE group_id=$1 AND id NOT IN(SELECT id FROM inflation_monitor_results WHERE group_id=$1 ORDER BY id DESC LIMIT 24)`, groupID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE inflation_monitor_plans SET lease_until=NULL,lease_token=NULL,next_run_at=NOW()+((config->>'interval_minutes')::int*INTERVAL '1 minute') WHERE group_id=$1`, groupID); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *InflationMonitorService) probeAccount(ctx context.Context, id int64, p InflationMonitorPlan) InflationMonitorSample {
	sample := InflationMonitorSample{AccountID: id, Model: p.Model}
	account, err := s.tester.accountRepo.GetByID(ctx, id)
	belongs := false
	if err == nil {
		for _, gid := range account.GroupIDs {
			if gid == p.GroupID {
				belongs = true
			}
		}
	}
	if !belongs || account.Status != "active" || (p.ReasoningEffort != "" && !account.IsOpenAI()) {
		sample.Error = "账号已停用或不再属于该分组"
		return sample
	}
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/internal/inflation-monitor", nil).WithContext(ctx)
	c.Set(accountTestBackgroundKey, true)
	err = s.tester.TestAccountConnection(c, id, p.Model, p.Prompt, "text", AccountTestOptions{MeasureTokens: true, TokenizerEncoding: p.Encoding, TimeoutSeconds: 120, ReasoningEffort: p.ReasoningEffort})
	if err != nil || ctx.Err() != nil || !successfulTestSSECompletion(w.Body.String()) {
		sample.Error = "检测未完整完成，未计入统计"
		return sample
	}
	probe := inflationProbe(c)
	if probe == nil {
		sample.Error = "未获取到检测数据"
		return sample
	}
	r := probe.result()
	if probe.model != "" {
		sample.Model = probe.model
	}
	sample.InputTokens = r.InputTokens
	sample.OutputTokens = r.OutputTokens
	sample.TextTokens = r.TokenCount
	sample.ReasoningTokens = r.ReasoningTokens
	if r.Error != "" || r.OutputTokens == nil || r.TokenCount <= 0 {
		sample.Error = "缺少输出用量或正文分词失败，未计入统计"
		return sample
	}
	rate := (float64(*r.OutputTokens)/float64(r.TokenCount) - 1) * 100
	sample.Rate = &rate
	return sample
}
func InflationAggregate(samples []InflationMonitorSample, mode string) *float64 {
	values := []float64{}
	for _, s := range samples {
		if s.Rate != nil && s.Error == "" {
			values = append(values, *s.Rate)
		}
	}
	if len(values) == 0 {
		return nil
	}
	var value float64
	if mode == "median" {
		sort.Float64s(values)
		n := len(values)
		value = values[n/2]
		if n%2 == 0 {
			value = (values[n/2-1] + value) / 2
		}
	} else {
		for _, v := range values {
			value += v
		}
		value /= float64(len(values))
	}
	return &value
}
func (s *InflationMonitorService) Summaries(ctx context.Context, userID int64) ([]InflationMonitorSummary, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT p.group_id,p.config,r.result FROM inflation_monitor_plans p LEFT JOIN LATERAL(SELECT result FROM inflation_monitor_results WHERE group_id=p.group_id ORDER BY id DESC LIMIT 1)r ON true WHERE EXISTS(SELECT 1 FROM api_keys k WHERE k.user_id=$1 AND k.group_id=p.group_id AND k.deleted_at IS NULL) ORDER BY p.group_id`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := []InflationMonitorSummary{}
	for rows.Next() {
		var id int64
		var config, raw []byte
		if err = rows.Scan(&id, &config, &raw); err != nil {
			return nil, err
		}
		var p InflationMonitorPlan
		if err = json.Unmarshal(config, &p); err != nil {
			return nil, err
		}
		item := InflationMonitorSummary{GroupID: id, Aggregation: p.Aggregation, YellowThreshold: p.YellowThreshold, RedThreshold: p.RedThreshold}
		if len(raw) > 0 {
			var r InflationMonitorResult
			if err = json.Unmarshal(raw, &r); err != nil {
				return nil, err
			}
			item.Rate = InflationAggregate(r.Samples, p.Aggregation)
			item.MeasuredAt = &r.MeasuredAt
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (s *InflationMonitorService) History(ctx context.Context, userID, keyID int64) (*InflationMonitorHistory, error) {
	var raw []byte
	var groupID int64
	err := s.db.QueryRowContext(ctx, `SELECT p.group_id,p.config FROM inflation_monitor_plans p JOIN api_keys k ON k.group_id=p.group_id WHERE k.id=$1 AND k.user_id=$2 AND k.deleted_at IS NULL`, keyID, userID).Scan(&groupID, &raw)
	if err != nil {
		return nil, err
	}
	var p InflationMonitorPlan
	if err = json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	result := &InflationMonitorHistory{GroupID: groupID, Aggregation: p.Aggregation, YellowThreshold: p.YellowThreshold, RedThreshold: p.RedThreshold, Results: []InflationMonitorResult{}}
	rows, err := s.db.QueryContext(ctx, `SELECT result FROM(SELECT id,result FROM inflation_monitor_results WHERE group_id=$1 ORDER BY id DESC LIMIT 24)t ORDER BY id`, groupID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var r InflationMonitorResult
		if err = json.Unmarshal(raw, &r); err != nil {
			return nil, err
		}
		for i := range r.Samples {
			r.Samples[i].AccountID = 0
		}
		result.Results = append(result.Results, r)
	}
	return result, rows.Err()
}
