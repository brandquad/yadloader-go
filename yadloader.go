package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

const (
	LIMIT     int = 100
	TIMEOUT   int = 10
	WAIT      int = 5
	MAXTRIES  int = 3
	CHUNKSIZE int = 1024
)

type entryType string

const (
	FILE entryType = "file"
	DIR  entryType = "dir"
)

type response struct {
	Path       string    `json:"path"`
	Type       entryType `json:"type"`
	Name       string    `json:"name"`
	Created    string    `json:"created"`
	Modified   string    `json:"modified"`
	Size       *int64    `json:"size"`
	MD5        *string   `json:"md5"`
	SHA256     *string   `json:"sha256"`
	PublicKey  string    `json:"public_key"`
	MediaType  *string   `json:"media_type"`
	ResourceId string    `json:"resource_id"`
	File       *string   `json:"file"`
	Embedded   *embedded `json:"_embedded"`
}

type embedded struct {
	Path   string     `json:"path"`
	Limit  int        `json:"limit"`
	Offset int        `json:"offset"`
	Sort   string     `json:"sort"`
	Total  int        `json:"total"`
	Items  []response `json:"items"`
}

type yadiskFile struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	File     string `json:"file"`
	MD5      string `json:"md5"`
	SHA256   string `json:"sha256"`
	Created  string `json:"created"`
	Modified string `json:"modified"`
}

func downloadFile(url, output string) error {
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = MAXTRIES
	retryClient.RetryWaitMin = time.Duration(WAIT) * time.Second

	resp, err := retryClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request failed with status code: %d", resp.StatusCode)
	}

	outFile, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("error creating output file: %w", err)
	}
	defer outFile.Close()

	buffer := make([]byte, CHUNKSIZE)
	for {
		n, err := resp.Body.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error reading response body: %w", err)
		}

		_, err = outFile.Write(buffer[:n])
		if err != nil {
			return fmt.Errorf("error writing to file: %w", err)
		}
	}

	return nil
}

func request(url string) (string, error) {
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = MAXTRIES
	retryClient.RetryWaitMin = time.Duration(WAIT) * time.Second

	resp, err := retryClient.Get(url)
	if err != nil {
		return "", err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil

}

func makeParams(a map[string]string) string {
	params := url.Values{}
	for k, v := range a {
		params.Add(k, v)
	}
	return params.Encode()
}

func getTree(link, path string) ([]yadiskFile, error) {
	if path == "" {
		path = "/"
	}
	var offset = 0
	var files []yadiskFile
	for {

		args := makeParams(map[string]string{
			"path":       path,
			"limit":      strconv.Itoa(LIMIT),
			"offset":     strconv.Itoa(offset),
			"public_key": link,
		})

		resp, err := request(fmt.Sprintf("https://cloud-api.yandex.net/v1/disk/public/resources?%s", args))
		if err != nil {
			return nil, err
		}

		var r response

		if err = json.Unmarshal([]byte(resp), &r); err != nil {
			return nil, err
		}

		if r.Embedded == nil || r.Embedded.Items == nil || len(r.Embedded.Items) == 0 {
			break
		}
		for _, i := range r.Embedded.Items {

			switch i.Type {
			case FILE:
				files = append(files, yadiskFile{
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
				subFiles, err := getTree(link, i.Path)
				if err != nil {
					return nil, err
				}
				if subFiles != nil {
					for _, f := range subFiles {
						files = append(files, yadiskFile{
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

		offset += LIMIT
		time.Sleep(time.Duration(TIMEOUT) * time.Second)
	}

	return files, nil
}

func downloadFiles(files []yadiskFile, output string) error {
	_, err := os.Stat(output)
	if os.IsNotExist(err) {
		err := os.MkdirAll(output, 0755)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err // Другая ошибка при проверке
	}
	for _, file := range files {
		finalPath := strings.TrimSuffix(file.Path, file.Name)
		finalFolder := filepath.Join(output, finalPath)
		if _, err = os.Stat(finalFolder); os.IsNotExist(err) {
			err := os.MkdirAll(finalFolder, 0755)
			if err != nil {
				return err
			}
		}

		finalPath = filepath.Join(finalFolder, file.Name)

		if err = downloadFile(file.File, finalPath); err != nil {
			log.Printf("error downloading file: %s, %s", file.Name, err)
		}

	}

	return nil
}

func main() {
	files, err := getTree("https://disk.yandex.ru/d/EWMbU0TAVn8fIA", "")
	if err != nil {
		panic(err)
	}

	var totalSize int64
	for _, file := range files {
		totalSize += file.Size
	}

	log.Printf("total size: %d", totalSize)
	log.Printf("total files: %d", len(files))

	//if err = downloadFiles(files, "output"); err != nil {
	//	panic(err)
	//}
}
