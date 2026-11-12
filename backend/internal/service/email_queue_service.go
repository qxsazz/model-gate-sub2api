package service

import (
	"context"
	"fmt"
	"html"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// Task type constants
const (
	TaskTypeVerifyCode    = "verify_code"
	TaskTypePasswordReset = "password_reset"
	TaskTypeChatBinding   = "chat_binding"
)

// EmailTask 邮件发送任务
type EmailTask struct {
	Email      string
	SiteName   string
	TaskType   string
	ResetURL   string // Only used for password_reset task type
	Locale     string // Optional Accept-Language locale hint
	GroupName  string
	BindingURL string
	ExpiresAt  time.Time
}

// EmailQueueService 异步邮件队列服务
type EmailQueueService struct {
	emailService *EmailService
	taskChan     chan EmailTask
	wg           sync.WaitGroup
	stopChan     chan struct{}
	workers      int
}

// NewEmailQueueService 创建邮件队列服务
func NewEmailQueueService(emailService *EmailService, workers int) *EmailQueueService {
	if workers <= 0 {
		workers = 3 // 默认3个工作协程
	}

	service := &EmailQueueService{
		emailService: emailService,
		taskChan:     make(chan EmailTask, 100), // 缓冲100个任务
		stopChan:     make(chan struct{}),
		workers:      workers,
	}

	// 启动工作协程
	service.start()

	return service
}

// start 启动工作协程
func (s *EmailQueueService) start() {
	for i := 0; i < s.workers; i++ {
		s.wg.Add(1)
		go s.worker(i)
	}
	logger.LegacyPrintf("service.email_queue", "[EmailQueue] Started %d workers", s.workers)
}

// worker 工作协程
func (s *EmailQueueService) worker(id int) {
	defer s.wg.Done()

	for {
		select {
		case task := <-s.taskChan:
			s.processTask(id, task)
		case <-s.stopChan:
			logger.LegacyPrintf("service.email_queue", "[EmailQueue] Worker %d stopping", id)
			return
		}
	}
}

// processTask 处理任务
func (s *EmailQueueService) processTask(workerID int, task EmailTask) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	switch task.TaskType {
	case TaskTypeVerifyCode:
		if err := s.emailService.SendVerifyCode(ctx, task.Email, task.SiteName, task.Locale); err != nil {
			logger.LegacyPrintf("service.email_queue", "[EmailQueue] Worker %d failed to send verify code to %s: %v", workerID, task.Email, err)
		} else {
			logger.LegacyPrintf("service.email_queue", "[EmailQueue] Worker %d sent verify code to %s", workerID, task.Email)
		}
	case TaskTypePasswordReset:
		if err := s.emailService.SendPasswordResetEmailWithCooldown(ctx, task.Email, task.SiteName, task.ResetURL, task.Locale); err != nil {
			logger.LegacyPrintf("service.email_queue", "[EmailQueue] Worker %d failed to send password reset to %s: %v", workerID, task.Email, err)
		} else {
			logger.LegacyPrintf("service.email_queue", "[EmailQueue] Worker %d sent password reset to %s", workerID, task.Email)
		}
	case TaskTypeChatBinding:
		if !time.Now().Before(task.ExpiresAt) {
			return
		}
		body := fmt.Sprintf(
			`<p>你收到了一笔「%s」的群聊绑定请求。</p><p>如果是你发起的，请登录 ModelGate 核对群名并选择是否绑定。</p><p><a href="%s">查看绑定请求</a></p><p>链接有效期为 10 分钟。</p>`,
			html.EscapeString(task.GroupName), html.EscapeString(task.BindingURL),
		)
		if err := s.emailService.SendEmail(ctx, task.Email, "ModelGate 群聊绑定确认", body); err != nil {
			logger.LegacyPrintf("service.email_queue", "[EmailQueue] Worker %d could not deliver a chat binding email", workerID)
		}
	default:
		logger.LegacyPrintf("service.email_queue", "[EmailQueue] Worker %d unknown task type: %s", workerID, task.TaskType)
	}
}

// EnqueueChatBindingEmail schedules delivery without exposing the address or URL in logs.
func (s *EmailQueueService) EnqueueChatBindingEmail(email, groupName, bindingURL string, expiresAt time.Time) error {
	select {
	case s.taskChan <- EmailTask{Email: email, TaskType: TaskTypeChatBinding, GroupName: groupName, BindingURL: bindingURL, ExpiresAt: expiresAt}:
		return nil
	default:
		return fmt.Errorf("email queue is full")
	}
}

// EnqueueVerifyCode 将验证码发送任务加入队列
func (s *EmailQueueService) EnqueueVerifyCode(email, siteName string, locale ...string) error {
	task := EmailTask{
		Email:    email,
		SiteName: siteName,
		TaskType: TaskTypeVerifyCode,
		Locale:   firstEmailLocale(locale),
	}

	select {
	case s.taskChan <- task:
		logger.LegacyPrintf("service.email_queue", "[EmailQueue] Enqueued verify code task for %s", email)
		return nil
	default:
		return fmt.Errorf("email queue is full")
	}
}

// EnqueuePasswordReset 将密码重置邮件任务加入队列
func (s *EmailQueueService) EnqueuePasswordReset(email, siteName, resetURL string, locale ...string) error {
	task := EmailTask{
		Email:    email,
		SiteName: siteName,
		TaskType: TaskTypePasswordReset,
		ResetURL: resetURL,
		Locale:   firstEmailLocale(locale),
	}

	select {
	case s.taskChan <- task:
		logger.LegacyPrintf("service.email_queue", "[EmailQueue] Enqueued password reset task for %s", email)
		return nil
	default:
		return fmt.Errorf("email queue is full")
	}
}

// Stop 停止队列服务
func (s *EmailQueueService) Stop() {
	close(s.stopChan)
	s.wg.Wait()
	logger.LegacyPrintf("service.email_queue", "%s", "[EmailQueue] All workers stopped")
}
