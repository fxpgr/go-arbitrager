package repository

type MessageRepository interface {
	Send(message string) (error)
}