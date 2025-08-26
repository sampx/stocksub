package tencent

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTencent_ParseTime_WithVariousFormats_ReturnsCorrectTime(t *testing.T) {
	testCases := []struct {
		name       string
		timeString string
		expected   time.Time
		isError    bool
	}{
		{
			name:       "标准14位格式",
			timeString: "20250821103000",
			expected:   time.Date(2025, 8, 21, 10, 30, 0, 0, time.Local),
		},
		{
			name:       "标准12位格式",
			timeString: "202508211030",
			expected:   time.Date(2025, 8, 21, 10, 30, 0, 0, time.Local),
		},
		{
			name:       "标准8位格式",
			timeString: "20250821",
			isError:    true, // The real parser returns time.Now() for this case
		},
		{
			name:       "无效格式",
			timeString: "2025-08-21 10:30:00",
			isError:    true, // The real parser returns time.Now() for this case
		},
		{
			name:       "长度错误",
			timeString: "2025082110",
			isError:    true, // The real parser returns time.Now() for this case
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parsedTime := parseTime(tc.timeString)
			if !tc.isError {
				assert.Equal(t, tc.expected, parsedTime, "解析结果不匹配")
			} else {
				// The real parseTime returns time.Now() on error, so we can't directly check for error.
				// We check if it's close to time.Now()
				assert.WithinDuration(t, time.Now(), parsedTime, 2*time.Second, "错误情况下应该返回当前时间")
			}
		})
	}
}
