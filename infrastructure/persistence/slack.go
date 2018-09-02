package persistence

import (
	"github.com/bluele/slack"
	"github.com/fxpgr/go-arbitrager/domain/repository"
	"github.com/fxpgr/go-arbitrager/infrastructure/logger"
	"go.uber.org/zap"
	"sync"
)

var mtx sync.Mutex

type slackClient struct {
	channel string
	client  *slack.Slack
	logger  *zap.SugaredLogger
}

func (s *slackClient) Send(message string) error {
	logger.Get().Info(message)
	return s.client.ChatPostMessage(s.channel, message, nil)
}

func (s *slackClient) BulkSend(messages []string) error {
	mtx.Lock()
	defer mtx.Unlock()
	slackMessage := "```"
	for _, m := range messages {
		slackMessage += m + "\n"
		s.logger.Info(m)
	}
	slackMessage += "```"
	return s.client.ChatPostMessage(s.channel, slackMessage, nil)
}

func NewSlackClient(token string, channel string, logger *zap.SugaredLogger) repository.MessageRepository {
	return &slackClient{channel, slack.New(token), logger}
}
