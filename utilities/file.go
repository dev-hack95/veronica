// Package utilities loads Veronica's runtime config (~/.veronica) and
// manages MCP server connections.
package utilities

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type ClientConfig struct {
	Model       string  `json:"model"`
	URL         string  `json:"url"`
	Role        string  `json:"role"`
	Stream      bool    `json:"stream"`
	Temperature float32 `json:"temperature"`
	TopP        float32 `json:"top_p"`
	MaxTokens   int     `json:"max_tokens"`
}

func CheckFileExist(filePath string) (ClientConfig, error) {
	var data ClientConfig

	homeDir, err1 := os.UserHomeDir()
	if err1 != nil {
		return data, err1
	}

	filePath = homeDir + filePath
	_, err := os.Stat(filePath)

	if err != nil {

		dirPath := filepath.Join(homeDir, ".veronica")

		err2 := os.MkdirAll(dirPath, 0755)

		if err2 != nil {
			return data, err2
		}

		filePath := filepath.Join(dirPath, "model.yaml")

		clientConfig := ClientConfig{
			Model:       "default",
			URL:         "http://localhost:8080",
			Role:        "user",
			Stream:      true,
			TopP:        0.7,
			Temperature: 0.8,
			MaxTokens:   32768,
		}

		yamlData, err3 := yaml.Marshal(&clientConfig)

		if err3 != nil {
			return data, err3
		}

		err4 := os.WriteFile(filePath, yamlData, 0755)

		if err4 != nil {
			return data, err4
		}
	}

	yamlFile, err := os.ReadFile(filePath)

	if err != nil {
		return data, err
	}

	err = yaml.Unmarshal(yamlFile, &data)

	if err != nil {
		return data, err
	}

	// Older versions wrote a misspelled "tempreature" key (yaml.v3 lowercases
	// untagged field names). Accept it when the corrected key is absent;
	// prefer "temperature" when both are present.
	var temps struct {
		Temperature *float32 `yaml:"temperature"`
		Tempreature *float32 `yaml:"tempreature"`
	}
	if err := yaml.Unmarshal(yamlFile, &temps); err != nil {
		return data, err
	}
	switch {
	case temps.Temperature != nil:
		data.Temperature = *temps.Temperature
	case temps.Tempreature != nil:
		data.Temperature = *temps.Tempreature
	}

	return data, nil
}
