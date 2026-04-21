package robot

// English note.
type MessageHandler interface {
	HandleMessage(platform, userID, text string) string
}
