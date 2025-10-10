package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"yadloader"
)

func makeFolder(folder string, perm os.FileMode) error {
	if perm == 0 {
		perm = 0755
	}
	_, err := os.Stat(folder)
	if os.IsNotExist(err) {
		err := os.MkdirAll(folder, perm)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err // Другая ошибка при проверке
	}
	return nil
}

type Args struct {
	Link   string
	Path   string
	Folder string
}

func parseFlags() *Args {
	config := &Args{}

	// Обязательный параметр
	flag.StringVar(&config.Link, "link", "", "Yandex.Disk public link (required)")
	flag.StringVar(&config.Link, "l", "", "Yandex.Disk public link (shorthand, required)")

	// Необязательный параметр
	flag.StringVar(&config.Path, "path", "", "Path to download (optional)")
	flag.StringVar(&config.Path, "p", "", "Path to download (shorthand, optional)")

	flag.StringVar(&config.Folder, "output", "", "Folder to download (optional)")
	flag.StringVar(&config.Folder, "o", "", "Folder to download (shorthand, optional)")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintln(flag.CommandLine.Output(), "Options:")

		flag.PrintDefaults()
		fmt.Fprintln(flag.CommandLine.Output(), "\nExamples:")
		fmt.Fprintln(flag.CommandLine.Output(), "  yadownload -l https://disk.yandex.ru/d/abc123")
		fmt.Fprintln(flag.CommandLine.Output(), "  yadownload --link https://disk.yandex.ru/d/abc123 --path /documents")
		fmt.Fprintln(flag.CommandLine.Output(), "  yadownload --link https://disk.yandex.ru/d/abc123 --path /documents --output download")
	}

	flag.Parse()

	// Проверка обязательного параметра
	if config.Link == "" {
		fmt.Fprintln(os.Stderr, "Error: link is required")
		flag.Usage()
		os.Exit(1)
	}

	return config
}

func main() {
	ctx := context.Background()
	params := parseFlags()

	client := yadloader.NewYaDiskClient(yadloader.NewDefaultConfig())
	//files, err := client.GetTree(ctx, "https://disk.yandex.ru/d/EWMbU0TAVn8fIA", "/Цифры/Вертикальные")
	files, err := client.GetTree(ctx, params.Link, params.Path)
	if err != nil {
		log.Fatal(err)
	}

	if params.Folder == "" {
		for _, file := range files {
			log.Println(file.Path, file.File)
		}
		os.Exit(0)
	}

	output := params.Folder
	if err := makeFolder(output, 0755); err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		finalPath := strings.TrimSuffix(file.Path, file.Name)
		finalFolder := filepath.Join(output, finalPath)
		if err := makeFolder(finalFolder, 0755); err != nil {
			log.Fatal(err)
		}

		finalPath = filepath.Join(finalFolder, file.Name)

		var download = func(path string) error {
			f, err := os.Create(finalPath)
			if err != nil {
				log.Fatal(err)
			}
			defer f.Close()
			if err := client.DownloadFile(ctx, file, f); err != nil {
				log.Fatal(err)
			}
			return nil
		}

		if err := download(finalPath); err != nil {
			log.Fatal(err)
		}

	}
}
