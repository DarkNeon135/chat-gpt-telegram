package openAI

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

const url = "https://api.openai.com/v1/completions"
const token = 2000
const temperature = 0.5
const model = "text-davinci-003"

func SendRequestToOpenAI(apiKey, message string, client http.Client) (string, error) {

	request := struct {
		Prompt      string  `json:"prompt"`
		MaxTokens   int     `json:"max_tokens"`
		Temperature float64 `json:"temperature"`
		Model       string  `json:"model"`
	}{
		Prompt:      message,
		MaxTokens:   token,
		Temperature: temperature,
		Model:       model,
	}
	jsonReq, _ := json.Marshal(request)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonReq))
	if err != nil {
		return "", fmt.Errorf("creating OpenAi request failed. Error: %s", err)
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("openAi request failed. Error: %s", err)
	}

	return checkGptResponse(res)
}

func checkGptResponse(res *http.Response) (string, error) {
	if res.StatusCode != 200 {

		gptError := struct {
			Error struct {
				Message string      `json:"message"`
				Type    string      `json:"type"`
				Param   interface{} `json:"param"`
				Code    interface{} `json:"code"`
			} `json:"error"`
		}{}

		err := json.NewDecoder(res.Body).Decode(&gptError)
		if err != nil {
			return "", fmt.Errorf("decoding json to struct failed. Error: %s", err)
		}
		return "", fmt.Errorf("request to OpenAI failed. Error: %s", gptError.Error.Message)
	}

	response := struct {
		Choices []struct {
			Text  string `json:"text"`
			Index int    `json:"index"`
		} `json:"choices"`
	}{}
	err := json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		return "", fmt.Errorf("decoding json to struct failed. Error: %s", err)
	}
	return response.Choices[0].Text, nil
}
