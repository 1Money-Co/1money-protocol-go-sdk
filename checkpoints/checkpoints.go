package checkpoints

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type CheckpointNumber struct {
	Number int `json:"number"`
}

func GetCheckpointNumber() (*CheckpointNumber, error) {
	gin.SetMode(gin.ReleaseMode)
	client := &http.Client{}

	req, err := http.NewRequest("GET", "https://api.testnet.1money.network/v1/checkpoints/number", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get checkpoint number: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result CheckpointNumber
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}
