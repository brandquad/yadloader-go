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

type GetTreeCallback func(count int64, totalSize int64)

type YaDiskClient struct {
	client *retryablehttp.Client
	config *Config
}

func NewYaDiskClient(config *Config) *YaDiskClient {
	retryClient := retryablehttp.NewClient()
	retryClient.RetryWaitMin = config.Wait
	retryClient.RetryMax = config.MaxTries
	retryClient.Logger = nil

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

func (c *YaDiskClient) GetTree(ctx context.Context, link, path string, cb ...GetTreeCallback) ([]diskFile, error) {
	if path == "" {
		path = "/"
	}

	var callback GetTreeCallback
	if len(cb) > 0 {
		callback = cb[0]
	}

	files := make([]diskFile, 0, c.config.Limit)
	var count int64
	var totalSize int64

	if err := c.getTree(ctx, link, path, &files, &count, &totalSize, callback); err != nil {
		return nil, err
	}
	return files, nil
}

func (c *YaDiskClient) getTree(
	ctx context.Context,
	link, path string,
	files *[]diskFile,
	count *int64,
	totalSize *int64,
	cb GetTreeCallback,
) error {
	offset := 0

	notify := func() {
		if cb != nil {
			cb(*count, *totalSize)
		}
	}

	for {
		args := c.makeParams(map[string]string{
			"path":       path,
			"limit":      strconv.Itoa(c.config.Limit),
			"offset":     strconv.Itoa(offset),
			"public_key": link,
		})

		resp, err := c.request(ctx, fmt.Sprintf("https://cloud-api.yandex.net/v1/disk/public/resources?%s", args))
		if err != nil {
			return err
		}

		var r response
		if err = json.Unmarshal(resp, &r); err != nil {
			return err
		}

		if r.Embedded == nil || r.Embedded.Items == nil || len(r.Embedded.Items) == 0 {
			break
		}

		for _, i := range r.Embedded.Items {
			switch i.Type {
			case FILE:
				*files = append(*files, diskFile{
					Name:     i.Name,
					Size:     *i.Size,
					File:     *i.File,
					Path:     i.Path,
					MD5:      *i.MD5,
					SHA256:   *i.SHA256,
					Created:  i.Created,
					Modified: i.Modified,
				})

				*count++
				*totalSize += int64(*i.Size)

				notify()

			case DIR:
				if err := c.getTree(ctx, link, i.Path, files, count, totalSize, cb); err != nil {
					return err
				}
			}
		}

		offset += c.config.Limit
		time.Sleep(c.config.Timeout)
	}

	return nil
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
