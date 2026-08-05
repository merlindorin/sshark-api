package v1

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/api/authenticated"
	"github.com/merlindorin/sshark-api/api/common"
	"github.com/merlindorin/sshark-api/internal/domain/tasks"
	"github.com/merlindorin/sshark-api/internal/infra/jobs"
)

// recentTaskLimit is how much history the UI is given. Enough to show what just happened
// without turning the endpoint into a log.
const recentTaskLimit = 20

// TaskServices holds what the task endpoints need.
type TaskServices struct {
	Tasks tasks.Repository
	Queue *jobs.Queue
}

// ListMyTasks returns the user's recent tasks.
func ListMyTasks(c *gin.Context, logger *zap.Logger, services TaskServices) {
	subject, ok := subjectFromContext(c)
	if !ok {
		return
	}

	entities, err := services.Tasks.ListByUser(c.Request.Context(), subject, recentTaskLimit)
	if err != nil {
		logger.Error("failed to list tasks", zap.Error(err))
		_ = c.Error(common.InternalError(c))
		return
	}

	response := make([]authenticated.Task, 0, len(entities))
	for i := range entities {
		response = append(response, toTask(&entities[i]))
	}

	c.JSON(http.StatusOK, gin.H{"tasks": response})
}

// GetMyTask returns one task, provided it belongs to the caller.
func GetMyTask(c *gin.Context, logger *zap.Logger, services TaskServices, id uuid.UUID) {
	subject, ok := subjectFromContext(c)
	if !ok {
		return
	}

	task, err := services.Tasks.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, tasks.ErrTaskNotFound) {
			_ = c.Error(common.TaskNotFoundError(c))
			return
		}
		logger.Error("failed to get task", zap.Error(err))
		_ = c.Error(common.InternalError(c))
		return
	}

	// Someone else's task is reported as missing rather than forbidden, so the endpoint does
	// not confirm that a task id exists.
	if task.ClerkUserID != subject {
		_ = c.Error(common.TaskNotFoundError(c))
		return
	}

	c.JSON(http.StatusOK, toTask(task))
}

func toTask(entity *tasks.Entity) authenticated.Task {
	task := authenticated.Task{
		Id:        entity.ID,
		Kind:      authenticated.TaskKind(entity.Kind),
		Status:    authenticated.TaskStatus(entity.Status),
		Progress:  entity.Progress,
		Total:     entity.Total,
		Message:   entity.Message,
		Error:     entity.Error,
		CreatedAt: entity.CreatedAt,
	}

	task.StartedAt = entity.StartedAt
	task.FinishedAt = entity.FinishedAt

	if len(entity.Result) > 0 {
		var result map[string]any
		if err := json.Unmarshal(entity.Result, &result); err == nil {
			task.Result = &result
		}
	}

	return task
}
