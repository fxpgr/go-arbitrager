package persistence

import (
	"github.com/bluele/slack"
	"github.com/fxpgr/go-arbitrager/domain/repository"
)

type slackClient struct {
	channel string
	client  *slack.Slack
}

func(s *slackClient) Send(message string) (error) {
	return s.client.ChatPostMessage(s.channel, message, nil)
}

func NewSlackClient(token string, channel string) repository.MessageRepository {
	return &slackClient{channel, slack.New(token)}
}