package utilities

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type ClientConfig struct {
	Model       string  `json:"model"`
	Url         string  `json:"url"`
	Role        string  `json:"role"`
	Stream      bool    `json:"stream"`
	Tempreature float32 `json:"tempreature"`
	TopP        float32 `json:"top_p"`
	MaxTokens   uint16  `json:"max_tokens"`
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

		file_path := filepath.Join(dirPath, "model.yaml")

		clientConfig := ClientConfig{
			Model:       "default",
			Url:         "http://localhost:8080",
			Role:        "user",
			Stream:      true,
			TopP:        0.7,
			Tempreature: 0.8,
			MaxTokens:   32768,
		}

		yamlData, err3 := yaml.Marshal(&clientConfig)

		if err3 != nil {
			return data, err3
		}

		err4 := os.WriteFile(file_path, yamlData, 0755)

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

	return data, nil
}
