package workers

import (
	"context"
	"time"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"

	"github.com/user/sync-tiktok-mps/internal/logger"
)

const (
	TaskProcessWebhook   = "webhook:process"
	TaskSyncInventory    = "inventory:sync"
	TaskShipPackage      = "fulfillment:ship"
	TaskReconcileOrders  = "orders:reconcile"
	TaskInitialShopSync  = "shop:initial_sync"
	TaskSendCallback     = "callback:send"
)

func NewAsynqClient(redisAddr string) *asynq.Client {
	return asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})
}

func NewAsynqServer(redisAddr string) *asynq.Server {
	return asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
			RetryDelayFunc: func(n int, e error, t *asynq.Task) time.Duration {
				return time.Duration(n*n) * time.Second
			},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				logger.Error("task processing failed",
					zap.String("task_type", task.Type()),
					zap.Error(err))
			}),
		},
	)
}

func RegisterHandlers(mux *asynq.ServeMux) {
	mux.HandleFunc(TaskProcessWebhook, HandleProcessWebhook)
	mux.HandleFunc(TaskSyncInventory, HandleSyncInventory)
	mux.HandleFunc(TaskShipPackage, HandleShipPackage)
	mux.HandleFunc(TaskReconcileOrders, HandleReconcileOrders)
	mux.HandleFunc(TaskInitialShopSync, HandleInitialShopSync)
	mux.HandleFunc(TaskSendCallback, HandleSendCallback)
}

func NewScheduler(redisAddr string) *asynq.Scheduler {
	return asynq.NewScheduler(
		asynq.RedisClientOpt{Addr: redisAddr},
		&asynq.SchedulerOpts{
			Location: time.Local,
		},
	)
}

func RegisterScheduledTasks(scheduler *asynq.Scheduler) error {
	_, err := scheduler.Register("0 */6 * * *", asynq.NewTask(TaskReconcileOrders, nil))
	if err != nil {
		return err
	}

	logger.Info("scheduled tasks registered: order reconciliation every 6 hours")
	return nil
}
