package events

import (
	"strings"
	"sync"
)

type Definition struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Group       string   `json:"group"`
	Status      string   `json:"status"`
	Fields      []string `json:"fields"`
	Description string   `json:"description"`
}

// Catalog is shared by producers, validation and the administrator's event selector.
// Reserved entries are visible but cannot accept rules until their business exists.
var catalogOnce sync.Once
var definitions []Definition
var definitionIndex map[string]Definition

func Catalog() []Definition { catalogOnce.Do(buildCatalog); return definitions }
func buildCatalog() {
	groups := []struct{ name, lines string }{
		{"用户与身份", `user.registered|注册完成
user.logged_in|登录成功
user.logged_out|退出登录
user.profile_updated|资料修改完成
user.password_changed|密码修改完成
user.password_reset|密码重置完成
user.identity_bound|身份绑定完成
user.identity_unbound|身份解绑完成
user.notify_email_verified|通知邮箱验证完成
user.notify_email_changed|通知邮箱状态改变
user.totp_changed|双重验证状态改变
user.passkey_changed|通行密钥变化
user.step_up_verified|敏感操作验证通过
user.mcp_decided|MCP 授权决定完成
user.mcp_revoked|MCP 授权撤销
user.token_refreshed|登录凭证刷新成功
user.verify_email_queued|验证邮件任务提交成功`},
		{"API Key", `key.created|API Key 创建完成
key.updated|API Key 配置改变
key.status_changed|API Key 启用或停用
key.group_changed|API Key 切换分组
key.deleted|API Key 删除完成
key.exhausted|API Key 额度耗尽
key.expired_detected|检测到 API Key 过期`},
		{"交易与兑换", `payment.created|支付订单创建完成
payment.paid|支付确认成功
payment.balance_completed|余额充值到账
payment.subscription_completed|订阅购买完成
payment.fulfillment_failed|订单权益发放失败
payment.cancelled|订单取消完成
payment.expired|待支付订单超时关闭
payment.refund_requested|退款申请提交
payment.refund_pending|退款等待确认
payment.refunded|退款完成
payment.refund_failed|退款失败
redeem.completed|兑换码使用成功
promo.applied|优惠码奖励到账
user.balance_adjusted|管理员调整余额`},
		{"订阅", `subscription.assigned|订阅获得
subscription.extended|订阅续期或延长
subscription.shortened|订阅时长缩短
subscription.revoked|订阅撤销
subscription.restored|订阅恢复
subscription.expired|订阅过期
subscription.window_activated|订阅用量窗口启用
subscription.window_reset|订阅用量窗口自动重置
subscription.quota_reset|管理员重置订阅额度
subscription.limit_detected|检测到订阅额度不足`},
		{"邀请与返佣", `affiliate.bound|邀请关系建立
affiliate.accrued|邀请返佣入账
affiliate.transferred|返佣转入余额
affiliate.withdrawn|管理员扣回返佣
affiliate.code_changed|邀请码改变
affiliate.rate_changed|专属返佣比例改变`},
		{"模型调用", `request.admitted|调用通过准入
request.queued|调用进入排队
request.forwarding|调用开始转发
request.first_output|收到首段有效输出
request.upstream_completed|上游调用完成
request.completed|用户请求完成
request.cancelled|客户端取消或断开
request.retried|调用发生重试
request.failover|调用切换账号或上游
request.no_account|没有可用账号
request.rate_limited|调用被限流
request.insufficient_quota|调用因额度不足被拒绝
request.safety_blocked|调用被内容安全拦截
request.realtime_started|实时会话建立
request.realtime_ended|实时会话结束`},
		{"计费", `billing.calculated|费用计算完成
billing.completed|用量结算完成
billing.failed|用量结算失败
billing.held|余额冻结完成
billing.captured|冻结余额结算完成
billing.released|冻结余额释放完成
billing.balance_low|余额跨过提醒线
billing.overdrafted|结算产生欠款
billing.window_limit|金额用量窗口达到限制`},
		{"媒体与批量图片", `media.created|异步生成任务创建
media.started|生成任务开始执行
media.succeeded|生成任务成功
media.failed|生成任务失败
media.recovery_required|生成任务需要恢复
media.files_saved|结果文件保存完成
media.files_failed|结果文件保存失败
media.downloaded|结果文件下载响应完成
media.deleted|用户删除媒体文件
media.cleaned|媒体文件自动清理
batch.submitted|批量图片任务提交完成
batch.item_completed|批量图片单项完成
batch.completed|批量图片整个批次完成
batch.cancelled|批量图片取消完成
batch.record_deleted|批量图片记录删除
batch.outputs_deleted|批量图片输出删除`},
		{"访问与互动", `announcement.read|公告标记已读
announcement.reacted|公告表情新增或改变
announcement.reaction_removed|公告表情取消
content.documentation|文档内容获取成功
content.skill_downloaded|Skill 下载响应完成
content.page|自定义页面内容获取成功
content.model_plaza|模型广场数据获取成功
content.apps|应用中心列表获取成功
content.user_view|用户业务页面数据获取成功
content.activity_view|积分及活动页面数据获取成功`},
		{"积分签到", `checkin.completed|每日签到完成
points.granted|积分增加
points.spent|积分消费
points.admin_granted|管理员积分充值完成
points.insufficient|积分消费余额不足`},
		{"抽奖任务", `lottery.participated|参加抽奖成功
lottery.participant_removed|抽奖参与资格被移除
lottery.drawn|抽奖开奖完成
lottery.won|用户中奖
lottery.reward_completed|抽奖奖励发放成功
lottery.reward_failed|抽奖奖励发放失败
lottery.closed|抽奖活动关闭
lottery.daily_created|每日抽奖活动生成
task.reward_created|用户任务奖励生成
task.reward_completed|用户任务奖励发放成功
task.reward_failed|用户任务奖励发放失败
reward.retried|奖励重试完成`},
		{"管理与系统", `admin.user_changed|管理员创建、修改或删除用户
admin.user_limits_changed|用户权限或限制改变
admin.user_attributes_changed|用户自定义属性改变
user.ban_changed|用户封禁或解封
admin.account_changed|上游账号管理操作完成
account.availability_changed|上游账号可用状态改变
account.quota_changed|上游账号额度状态改变
monitor.completed|渠道检测完成
ops.alert_created|系统告警产生
ops.alert_resolved|系统告警恢复或关闭
model.test_completed|模型检测任务完成
backup.completed|备份任务结束
backup.restore_completed|恢复备份任务结束
email.completed|邮件发送完成或失败
admin.content_changed|运营内容改变
admin.settings_changed|系统配置改变`},
		{"累计与时间", `user.first_login|首次登录
user.first_call|首次完成调用
user.first_recharge|首次充值
user.daily_active|每日首次活跃
user.daily_spend|每日首次消费
user.active_streak|连续活跃天数变化
user.returned|间隔多日后回归
checkin.cycle_completed|签到周期完成
usage.requests_updated|累计请求次数变化
usage.tokens_updated|累计 Token 变化
usage.cost_updated|累计消费变化
usage.recharge_updated|累计充值变化
usage.models_updated|使用模型或分组种类变化
affiliate.count_updated|邀请人数变化
request.streak_updated|连续调用成功或失败次数变化
lottery.loss_streak_updated|连续未中奖次数变化
user.age_updated|注册天数变化
usage.tier_changed|消费阶梯或折扣等级改变
balance.threshold_changed|余额积分返佣数值变化
schedule.upcoming|订阅、Key、活动即将到期或开始`},
	}
	fields := strings.Fields("user_id actor_id inviter_id invitee_id group_id api_key_id account_id model platform status action resource source_id order_type provider amount pay_amount currency actual_cost total_cost input_tokens output_tokens total_tokens image_count duration_ms duration_seconds count daily_count total total_amount total_tokens_used unique_models unique_groups streak days day multiplier reward_points balance balance_after quantity item_id item_code reason http_status endpoint method previous_status expires_at registered_at subscription_id activity_id task_id batch_id event_count event_daily_count event_total stage previous_stage rate_multiplier revision concurrency rpm_limit role")
	result := make([]Definition, 0, 160)
	for _, g := range groups {
		for _, line := range strings.Split(g.lines, "\n") {
			p := strings.SplitN(line, "|", 2)
			result = append(result, Definition{Code: p[0], Name: p[1], Group: g.name, Status: "active", Fields: fields})
		}
	}
	for _, p := range [][2]string{{"shop.redeemed", "商城兑换成功"}, {"item.activated", "道具启用"}, {"item.expired", "道具失效"}, {"shield.triggered", "护盾触发"}, {"client.interaction", "前端停留或外部应用使用"}} {
		result = append(result, Definition{Code: p[0], Name: p[1], Group: "待业务接入", Status: "reserved", Fields: fields, Description: "对应业务尚未实现，暂不能启用规则"})
	}
	result = append(result, Definition{Code: "item.granted", Name: "道具获得", Group: "道具", Status: "active", Fields: fields}, Definition{Code: "item.used", Name: "道具使用完成", Group: "道具", Status: "active", Fields: fields})
	result = append(result, Definition{Code: "usage.tier_observed", Name: "消费阶梯观察", Group: "内部数据", Status: "internal", Fields: fields})
	definitions = result
	definitionIndex = make(map[string]Definition, len(result))
	for _, d := range result {
		definitionIndex[d.Code] = d
	}
}
func Lookup(code string) (Definition, bool) {
	catalogOnce.Do(buildCatalog)
	d, ok := definitionIndex[code]
	return d, ok
}
