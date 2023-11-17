package logger

type Options struct {
	Level string
}

type Logger struct {
	LevelLogger int
}

func NewLogger(option Options) *Logger {
	return &Logger{
		LevelLogger: getLevel(option.Level),
	}
}

func (l *Logger) Info(message string) {
	if l.LevelLogger <= 1 {
		println(message)
	}
}

func (l *Logger) Debug(message string) {
	if l.LevelLogger <= 2 {
		println(message)
	}
}

func (l *Logger) Error(message string) {
	if l.LevelLogger <= 3 {
		println(message)
	}
}

func getLevel(level string) int {
	switch level {
	case "info":
		return 1
	case "debug":
		return 2
	case "error":
		return 3
	default:
		return 1
	}
}
