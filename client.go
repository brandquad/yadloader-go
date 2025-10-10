package yadloader

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

type Config struct {
	Limit     int
	Timeout   time.Duration
	Wait      time.Duration
	MaxTries  int
	ChunkSize int
}

func NewDefaultConfig() *Config {
	return &Config{
		Limit:     100,
		Timeout:   10 * time.Second,
		Wait:      5 * time.Second,
		MaxTries:  3,
		ChunkSize: 1024 * 1024, // 1MB
	}
}

type YaDiskClient struct {
	client *retryablehttp.Client
	config *Config
}

func NewYaDiskClient(config *Config) *YaDiskClient {
	retryClient := retryablehttp.NewClient()
	retryClient.RetryWaitMin = config.Wait
	retryClient.RetryMax = config.MaxTries
	return &YaDiskClient{
		client: retryClient,
		config: config,
	}
}

func (c *YaDiskClient) makeParams(a map[string]string) string {
	params := url.Values{}
	for k, v := range a {
		params.Add(k, v)
	}
	return params.Encode()
}

func (c *YaDiskClient) request(ctx context.Context, url string) ([]byte, error) {

	req, err := retryablehttp.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (c *YaDiskClient) GetTree(ctx context.Context, link, path string) ([]diskFile, error) {

	if path == "" {
		path = "/"
	}
	var offset = 0
	var files []diskFile

	for {

		args := c.makeParams(map[string]string{
			"path":       path,
			"limit":      strconv.Itoa(c.config.Limit),
			"offset":     strconv.Itoa(offset),
			"public_key": link,
		})

		resp, err := c.request(ctx, fmt.Sprintf("https://cloud-api.yandex.net/v1/disk/public/resources?%s", args))
		if err != nil {
			return nil, err
		}

		var r response

		if err = json.Unmarshal(resp, &r); err != nil {
			return nil, err
		}

		if r.Embedded == nil || r.Embedded.Items == nil || len(r.Embedded.Items) == 0 {
			break
		}
		for _, i := range r.Embedded.Items {

			switch i.Type {
			case FILE:
				files = append(files, diskFile{
					Name:     i.Name,
					Size:     *i.Size,
					File:     *i.File,
					Path:     i.Path,
					MD5:      *i.MD5,
					SHA256:   *i.SHA256,
					Created:  i.Created,
					Modified: i.Modified,
				})
			case DIR:
				subFiles, err := c.GetTree(ctx, link, i.Path)
				if err != nil {
					return nil, err
				}
				if subFiles != nil {
					for _, f := range subFiles {
						files = append(files, diskFile{
							Name:     f.Name,
							Size:     f.Size,
							File:     f.File,
							Path:     f.Path,
							MD5:      f.MD5,
							SHA256:   f.SHA256,
							Created:  f.Created,
							Modified: f.Modified,
						})
					}
				}
			}
		}

		offset += c.config.Limit
		time.Sleep(c.config.Timeout)
	}

	return files, nil
}

func (c *YaDiskClient) DownloadFile(ctx context.Context, file diskFile, writer io.Writer) error {
	req, err := retryablehttp.NewRequestWithContext(ctx, "GET", file.File, nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	buffer := make([]byte, c.config.ChunkSize)
	_, err = io.CopyBuffer(writer, resp.Body, buffer)
	if err != nil {
		return err
	}
	return nil
}
