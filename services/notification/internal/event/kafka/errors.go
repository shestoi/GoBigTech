package kafka

// ParseError представляет ошибку парсинга события
type ParseError struct {
	Field   string
	Message string
}

func (e *ParseError) Error() string {
	return e.Message
}
