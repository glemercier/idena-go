package config

import (
	"time"
)

type ValidationConfig struct {
	ValidationInterval       time.Duration
	FlipLotteryDuration      time.Duration
	ShortSessionDuration     time.Duration
	LongSessionDuration      time.Duration
	AfterLongSessionDuration time.Duration
}

func GetDefaultValidationConfig() *ValidationConfig {
	return &ValidationConfig{
		ValidationInterval:       time.Minute * 20,
		FlipLotteryDuration:      time.Second * 30,
		ShortSessionDuration:     time.Minute * 1,
		LongSessionDuration:      time.Minute * 1,
		AfterLongSessionDuration: time.Second * 30,
	}
}
