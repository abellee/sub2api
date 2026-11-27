package service

import (
	"context"
	"lotteryd/api/events"
)

func publishImageTaskEvent(kind string, task *ImageTaskRecord) {
	if task == nil || !events.Enabled() {
		return
	}
	events.Publish(kind, task.UserID, task.ID, map[string]string{"source_id": task.ID, "api_key_id": events.S(task.APIKeyID), "group_id": events.S(task.GroupID), "model": task.Model, "platform": task.Platform, "status": task.Status, "http_status": events.S(task.HTTPStatus)})
}

func publishBatchGameplayEvent(kind string, job *BatchImageJob, reference string, extra map[string]string) {
	if job == nil || !events.Enabled() {
		return
	}
	data := map[string]string{"batch_id": job.BatchID, "source_id": job.BatchID, "model": job.Model, "provider": job.Provider, "status": job.Status, "count": events.S(job.ItemCount), "image_count": events.S(job.SuccessCount)}
	if job.APIKeyID != nil {
		data["api_key_id"] = events.S(*job.APIKeyID)
	}
	if job.AccountID != nil {
		data["account_id"] = events.S(*job.AccountID)
	}
	for k, v := range extra {
		data[k] = v
	}
	if reference == "" {
		reference = job.BatchID
	}
	events.Publish(kind, job.UserID, reference, data)
}

func (s *BatchImagePublicService) recordGameplaySubmitFailure(ctx context.Context, job *BatchImageJob, code, message string) {
	if err := s.Repo.RecordBatchImageJobSubmitFailure(ctx, job.BatchID, code, message, true); err == nil {
		publishBatchGameplayEvent("media.failed", job, "", map[string]string{"status": BatchImageJobStatusFailed, "reason": code})
	}
}
