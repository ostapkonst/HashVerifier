package stats

type VerifyStatusType int

const (
	HashMatched VerifyStatusType = iota
	HashMismatch
	Unreadable
)

func (v VerifyStatusType) String() string {
	switch v {
	case HashMatched:
		return "MATCHED"
	case HashMismatch:
		return "MISMATCH"
	case Unreadable:
		return "UNREADABLE"
	default:
		panic("unknown status")
	}
}

func (v VerifyStatusType) Priority() int {
	switch v {
	case HashMatched:
		return 0
	case Unreadable:
		return 1
	case HashMismatch:
		return 2
	default:
		return 3
	}
}

func (v VerifyStatusType) Color() string {
	switch v {
	case HashMatched:
		return "green"
	case Unreadable:
		return "dark orange"
	case HashMismatch:
		return "firebrick1"
	default:
		return ""
	}
}

// GenerateStatusType — статус генерации хеша для одного файла.
type GenerateStatusType int

const (
	GenSuccess GenerateStatusType = iota
	GenSkipped
	GenFailed
)

func (g GenerateStatusType) String() string {
	switch g {
	case GenSuccess:
		return "SUCCESS"
	case GenSkipped:
		return "SKIPPED"
	case GenFailed:
		return "FAILED"
	default:
		panic("unknown status")
	}
}

func (g GenerateStatusType) Priority() int {
	switch g {
	case GenSuccess:
		return 0
	case GenSkipped:
		return 1
	case GenFailed:
		return 2
	default:
		return 3
	}
}

func (g GenerateStatusType) Color() string {
	switch g {
	case GenSuccess:
		return "green"
	case GenSkipped:
		return "gray"
	case GenFailed:
		return "firebrick1"
	default:
		return ""
	}
}

// VerifyResult — результат проверки одного файла.
type VerifyResult struct {
	Path         string           // относительный путь
	FullPath     string           // полный путь
	ActualHash   string           // вычисленный хеш
	ExpectedHash string           // ожидаемый хеш
	Status       VerifyStatusType // статус сравнения хешей
	ReadBytes    int64            // количество прочитанных байт файла при вычислении хеша
	Err          error            // ошибка при вычислении хеша
}

// GenerateResult — результат генерации хеша для одного файла.
type GenerateResult struct {
	RelPath   string             // относительный путь с префиксом или без него
	FullPath  string             // полный путь
	Hash      string             // вычисленный хеш
	ReadBytes int64              // количество прочитанных байт файла при вычислении хеша
	Err       error              // ошибка при вычислении хеша
	Status    GenerateStatusType // статус генерации хеша
}

// Статистика для генератора.
type GeneratorStats struct {
	TotalFiles          int     // всего файлов в чек-сумме
	Processed           int     // обработано успешно
	WithErrors          int     // не удалось обработать
	Skipped             int     // пропущено (исключено пользователем или некорректное имя файла для checksum-формата)
	CurrentFileOrStatus string  // текущий файл или статус
	FileHashingProgress float64 // прогресс вычисления хеша текущего файла
	Speed               float64 // скорость хеширования в байтах/сек
}

func NewGeneratorStats() GeneratorStats {
	return GeneratorStats{
		CurrentFileOrStatus: "ready to go...",
	}
}

func (g GeneratorStats) Pending() int {
	return g.TotalFiles - g.Processed - g.WithErrors - g.Skipped
}

func (g GeneratorStats) TotalProgress() float64 {
	if g.TotalFiles == 0 {
		return 0
	}

	return float64(g.TotalFiles-g.Pending()) / float64(g.TotalFiles)
}

// Статистика для верификатора.
type VerifierStats struct {
	TotalFiles          int     // всего файлов в чек-сумме
	Matched             int     // проверено успешно
	Mismatch            int     // не прошло проверку
	Unreadable          int     // не удалось проверить
	CurrentFileOrStatus string  // текущий файл или статус
	FileHashingProgress float64 // прогресс вычисления хеша текущего файла
	Speed               float64 // скорость хеширования в байтах/сек
}

func NewVerifierStats() VerifierStats {
	return VerifierStats{
		CurrentFileOrStatus: "ready to go...",
	}
}

func (v VerifierStats) Pending() int { return v.TotalFiles - v.Matched - v.Mismatch - v.Unreadable }

func (v VerifierStats) TotalProgress() float64 {
	if v.TotalFiles == 0 {
		return 0
	}

	return float64(v.TotalFiles-v.Pending()) / float64(v.TotalFiles)
}
