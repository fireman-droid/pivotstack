package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	BaseURL  string
	Password string
	HTTP     *http.Client
}

func New(baseURL, password string) *Client {
	return &Client{
		BaseURL:  baseURL,
		Password: password,
		HTTP:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) request(method, path string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Admin-Password", c.Password)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return resp, nil
}

func (c *Client) GetAccounts() ([]map[string]interface{}, error) {
	resp, err := c.request("GET", "/admin/api/accounts", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var accounts []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&accounts); err != nil {
		return nil, err
	}
	return accounts, nil
}

func (c *Client) RefreshAccount(id string) error {
	resp, err := c.request("POST", "/admin/api/accounts/"+id+"/refresh", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *Client) DeleteAccount(id string) error {
	resp, err := c.request("DELETE", "/admin/api/accounts/"+id, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *Client) UpdateAccount(id string, updates map[string]interface{}) error {
	resp, err := c.request("PUT", "/admin/api/accounts/"+id, updates)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *Client) GetStatus() (map[string]interface{}, error) {
	resp, err := c.request("GET", "/admin/api/status", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var status map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}
	return status, nil
}

func (c *Client) GetLogs(limit int) ([]map[string]interface{}, error) {
	path := "/admin/api/logs"
	if limit > 0 {
		path = fmt.Sprintf("%s?limit=%d", path, limit)
	}

	resp, err := c.request("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var logs []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&logs); err != nil {
		return nil, err
	}
	return logs, nil
}
