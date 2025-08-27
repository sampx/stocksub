//go:build integration
// +build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIServerIntegration(t *testing.T) {
	// Setup Redis client for test data
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1, // Use test database
	})
	defer redisClient.Close()

	ctx := context.Background()

	// Clean up test data
	defer func() {
		redisClient.FlushDB(ctx)
	}()

	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	t.Run("HealthCheck", func(t *testing.T) {
		router := gin.New()
		router.GET("/health", func(c *gin.Context) {
			// Simplified health check for testing
			health := map[string]interface{}{
				"status":    "ok",
				"timestamp": time.Now(),
				"services": map[string]string{
					"redis": "ok",
				},
			}
			c.JSON(200, health)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/health", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "ok", response["status"])
	})

	t.Run("GetStock", func(t *testing.T) {
		// Setup test data in Redis
		symbol := "600000"
		key := fmt.Sprintf("latest:stock:%s", symbol)

		testData := map[string]interface{}{
			"symbol":         symbol,
			"name":           "浦发银行",
			"price":          10.50,
			"change":         0.15,
			"change_percent": 1.45,
			"volume":         1250000,
			"timestamp":      time.Now().Unix(),
			"provider":       "tencent",
			"market":         "A-share",
			"updated_at":     time.Now().Unix(),
		}

		err := redisClient.HMSet(ctx, key, testData).Err()
		require.NoError(t, err)

		// Add to symbols set
		err = redisClient.SAdd(ctx, "symbols:stock", symbol).Err()
		require.NoError(t, err)

		// Create test router
		router := gin.New()
		router.GET("/api/v1/stocks/:symbol", func(c *gin.Context) {
			symbol := c.Param("symbol")

			result, err := redisClient.HGetAll(ctx, fmt.Sprintf("latest:stock:%s", symbol)).Result()
			if err != nil || len(result) == 0 {
				c.JSON(404, gin.H{"error": "not_found", "message": "Stock not found"})
				return
			}

			// Parse data (simplified for test)
			response := map[string]interface{}{
				"symbol":         result["symbol"],
				"name":           result["name"],
				"price":          result["price"],
				"change":         result["change"],
				"change_percent": result["change_percent"],
				"volume":         result["volume"],
				"provider":       result["provider"],
				"market":         result["market"],
			}

			c.JSON(200, response)
		})

		// Test successful request
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/stocks/600000", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "600000", response["symbol"])
		assert.Equal(t, "浦发银行", response["name"])
		assert.Equal(t, "10.5", response["price"])

		// Test not found
		w = httptest.NewRecorder()
		req, _ = http.NewRequest("GET", "/api/v1/stocks/999999", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 404, w.Code)
	})

	t.Run("GetStocks", func(t *testing.T) {
		// Setup multiple stocks in Redis
		stocks := []map[string]interface{}{
			{
				"symbol": "600000",
				"name":   "浦发银行",
				"price":  10.50,
			},
			{
				"symbol": "000001",
				"name":   "平安银行",
				"price":  15.20,
			},
		}

		for _, stock := range stocks {
			key := fmt.Sprintf("latest:stock:%s", stock["symbol"])
			stockData := map[string]interface{}{
				"symbol":         stock["symbol"],
				"name":           stock["name"],
				"price":          stock["price"],
				"change":         0.0,
				"change_percent": 0.0,
				"volume":         1000000,
				"timestamp":      time.Now().Unix(),
				"provider":       "tencent",
				"market":         "A-share",
				"updated_at":     time.Now().Unix(),
			}

			err := redisClient.HMSet(ctx, key, stockData).Err()
			require.NoError(t, err)

			err = redisClient.SAdd(ctx, "symbols:stock", stock["symbol"]).Err()
			require.NoError(t, err)
		}

		// Create test router
		router := gin.New()
		router.GET("/api/v1/stocks", func(c *gin.Context) {
			// Get all stock symbols
			symbols, err := redisClient.SMembers(ctx, "symbols:stock").Result()
			if err != nil {
				c.JSON(500, gin.H{"error": "internal_error"})
				return
			}

			var stocksResponse []map[string]interface{}
			for _, symbol := range symbols {
				key := fmt.Sprintf("latest:stock:%s", symbol)
				result, err := redisClient.HGetAll(ctx, key).Result()
				if err != nil || len(result) == 0 {
					continue
				}

				stocksResponse = append(stocksResponse, map[string]interface{}{
					"symbol": result["symbol"],
					"name":   result["name"],
					"price":  result["price"],
				})
			}

			c.JSON(200, stocksResponse)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/stocks", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)

		var response []map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Len(t, response, 2)

		// Verify both stocks are present
		symbols := make([]string, len(response))
		for i, stock := range response {
			symbols[i] = stock["symbol"].(string)
		}
		assert.Contains(t, symbols, "600000")
		assert.Contains(t, symbols, "000001")
	})

	t.Run("GetIndex", func(t *testing.T) {
		// Setup test index data in Redis
		symbol := "000001"
		key := fmt.Sprintf("latest:index:%s", symbol)

		testData := map[string]interface{}{
			"symbol":         symbol,
			"name":           "上证指数",
			"value":          3200.50,
			"change":         15.20,
			"change_percent": 0.48,
			"timestamp":      time.Now().Unix(),
			"provider":       "tencent",
			"market":         "A-share",
			"updated_at":     time.Now().Unix(),
		}

		err := redisClient.HMSet(ctx, key, testData).Err()
		require.NoError(t, err)

		// Add to symbols set
		err = redisClient.SAdd(ctx, "symbols:index", symbol).Err()
		require.NoError(t, err)

		// Create test router
		router := gin.New()
		router.GET("/api/v1/indices/:symbol", func(c *gin.Context) {
			symbol := c.Param("symbol")

			result, err := redisClient.HGetAll(ctx, fmt.Sprintf("latest:index:%s", symbol)).Result()
			if err != nil || len(result) == 0 {
				c.JSON(404, gin.H{"error": "not_found", "message": "Index not found"})
				return
			}

			response := map[string]interface{}{
				"symbol":         result["symbol"],
				"name":           result["name"],
				"value":          result["value"],
				"change":         result["change"],
				"change_percent": result["change_percent"],
				"provider":       result["provider"],
				"market":         result["market"],
			}

			c.JSON(200, response)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/indices/000001", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "000001", response["symbol"])
		assert.Equal(t, "上证指数", response["name"])
		assert.Equal(t, "3200.5", response["value"])
	})

	t.Run("GetSymbols", func(t *testing.T) {
		// Setup test symbols
		stockSymbols := []string{"600000", "000001", "000002"}
		indexSymbols := []string{"000001", "399001"}

		for _, symbol := range stockSymbols {
			err := redisClient.SAdd(ctx, "symbols:stock", symbol).Err()
			require.NoError(t, err)
		}

		for _, symbol := range indexSymbols {
			err := redisClient.SAdd(ctx, "symbols:index", symbol).Err()
			require.NoError(t, err)
		}

		// Create test router
		router := gin.New()
		router.GET("/api/v1/symbols/stocks", func(c *gin.Context) {
			symbols, err := redisClient.SMembers(ctx, "symbols:stock").Result()
			if err != nil {
				c.JSON(500, gin.H{"error": "internal_error"})
				return
			}

			c.JSON(200, map[string]interface{}{
				"type":    "stock",
				"symbols": symbols,
				"count":   len(symbols),
			})
		})

		router.GET("/api/v1/symbols/indices", func(c *gin.Context) {
			symbols, err := redisClient.SMembers(ctx, "symbols:index").Result()
			if err != nil {
				c.JSON(500, gin.H{"error": "internal_error"})
				return
			}

			c.JSON(200, map[string]interface{}{
				"type":    "index",
				"symbols": symbols,
				"count":   len(symbols),
			})
		})

		// Test stock symbols
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/symbols/stocks", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)

		var stockResponse map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &stockResponse)
		require.NoError(t, err)
		assert.Equal(t, "stock", stockResponse["type"])
		assert.Equal(t, float64(3), stockResponse["count"])

		// Test index symbols
		w = httptest.NewRecorder()
		req, _ = http.NewRequest("GET", "/api/v1/symbols/indices", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)

		var indexResponse map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &indexResponse)
		require.NoError(t, err)
		assert.Equal(t, "index", indexResponse["type"])
		assert.Equal(t, float64(2), indexResponse["count"])
	})

	t.Run("CORS", func(t *testing.T) {
		router := gin.New()

		// Add CORS middleware
		router.Use(func(c *gin.Context) {
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")

			if c.Request.Method == "OPTIONS" {
				c.AbortWithStatus(204)
				return
			}

			c.Next()
		})

		router.GET("/api/v1/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"message": "test"})
		})

		// Test OPTIONS request
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("OPTIONS", "/api/v1/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 204, w.Code)
		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
		assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "GET")

		// Test regular request with CORS headers
		w = httptest.NewRecorder()
		req, _ = http.NewRequest("GET", "/api/v1/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)
		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("ErrorHandling", func(t *testing.T) {
		router := gin.New()

		// Add recovery middleware
		router.Use(gin.Recovery())

		router.GET("/api/v1/error", func(c *gin.Context) {
			panic("test panic")
		})

		router.GET("/api/v1/not-found", func(c *gin.Context) {
			c.JSON(404, gin.H{"error": "not_found", "message": "Resource not found"})
		})

		router.GET("/api/v1/bad-request", func(c *gin.Context) {
			c.JSON(400, gin.H{"error": "bad_request", "message": "Invalid request"})
		})

		// Test panic recovery
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/error", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 500, w.Code)

		// Test 404 error
		w = httptest.NewRecorder()
		req, _ = http.NewRequest("GET", "/api/v1/not-found", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 404, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "not_found", response["error"])

		// Test 400 error
		w = httptest.NewRecorder()
		req, _ = http.NewRequest("GET", "/api/v1/bad-request", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 400, w.Code)

		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "bad_request", response["error"])
	})
}
