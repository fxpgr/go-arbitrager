package repository

type MessageRepository interface {
	Send(messages string) error
	BulkSend(messages []string) error
}
