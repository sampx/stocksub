package sina

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGbkToUtf8(t *testing.T) {
	gbkBytes := []byte{0xc6, 0xd6, 0xb7, 0xa2, 0xd2, 0xf8, 0xd0, 0xd0} // "浦发银行" in GBK
	utf8Str := gbkToUtf8(string(gbkBytes))
	assert.Equal(t, "浦发银行", utf8Str)
}

func TestSinaProvider_FetchStockData_Success(t *testing.T) {
	// 模拟新浪服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求是否正确
		assert.Equal(t, "/list=sh600000,sz000001", r.URL.RequestURI())
		assert.Equal(t, "https://finance.sina.com.cn/", r.Header.Get("Referer"))

		// 直接构造包含 GBK 字节的响应体
		var body bytes.Buffer
		body.WriteString(`var hq_str_sh600000="`)
		body.Write([]byte{0xc6, 0xd6, 0xb7, 0xa2, 0xd2, 0xf8, 0xd0, 0xd0}) // 浦发银行
		body.WriteString(`,10.500,10.450,10.550,10.600,10.400,10.540,10.550,1234500,12962250.00,100,10.54,200,10.53,300,10.52,400,10.51,500,10.50,100,10.55,200,10.56,300,10.57,400,10.58,500,10.59,2024-08-27,14:30:00,00";"`)
		body.WriteString("\n")
		body.WriteString(`var hq_str_sz000001="`)
		body.Write([]byte{0xc6, 0xbd, 0xb0, 0xb2, 0xd2, 0xf8, 0xd0, 0xd0}) // 平安银行
		body.WriteString(`,12.800,12.750,12.850,12.900,12.700,12.840,12.850,5432100,69530885.00,100,12.84,200,12.83,300,12.82,400,12.81,500,12.80,100,12.85,200,12.86,300,12.87,400,12.88,500,12.89,2024-08-27,14:30:00,00";`)

		w.Header().Set("Content-Type", "application/javascript; charset=GBK")
		_, _ = w.Write(body.Bytes())
	}))
	defer server.Close()

	// 创建使用模拟服务器的 Provider
	provider := NewProvider()
	provider.httpClient = server.Client()
	provider.baseURL = server.URL + "/list=" // 指向测试服务器

	// 执行测试
	symbols := []string{"600000", "000001"}
	data, err := provider.FetchStockData(context.Background(), symbols)

	// 断言结果
	assert.NoError(t, err)
	assert.Len(t, data, 2)

	// 验证第一支股票 (浦发银行)
	stock1 := data[0]
	assert.Equal(t, "600000", stock1.Symbol)
	assert.Equal(t, "浦发银行", stock1.Name)
	assert.InDelta(t, 10.550, stock1.Price, 0.001)

	// 验证第二支股票 (平安银行)
	stock2 := data[1]
	assert.Equal(t, "000001", stock2.Symbol)
	assert.Equal(t, "平安银行", stock2.Name)
}

func TestParseSinaData_EmptyInput(t *testing.T) {
	data := parseSinaData("")
	assert.Empty(t, data)
}

func TestParseSinaData_InvalidFormat(t *testing.T) {
	// 无效的行格式
	input := `var hq_str_sh600000="";`
	data := parseSinaData(input)
	assert.Empty(t, data)

	// 字段数量不足
	input = `var hq_str_sh600001="部分数据,1,2,3";`
	data = parseSinaData(input)
	assert.Empty(t, data)
}
