package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func init() {
	// zerolog initialization
	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	output.FormatLevel = func(i interface{}) string {
		return strings.ToUpper(fmt.Sprintf("| %-6s|", i))
	}
	output.FormatMessage = func(i interface{}) string {
		return fmt.Sprintf("***%s****", i)
	}
	output.FormatFieldName = func(i interface{}) string {
		return fmt.Sprintf("%s:", i)
	}
	output.FormatFieldValue = func(i interface{}) string {
		return strings.ToUpper(fmt.Sprintf("%s", i))
	}

	log.Logger = zerolog.New(output).With().Timestamp().Logger()

	// flag initialization
	path = flag.String("path", ".", "path to watch")
	device = flag.String("device", "my-device", "device name")
	level = flag.String("level", "off", "log level: debug, info, warn, error, off")
	flag.Parse()
}

var (
	path   *string
	device *string
	level  *string
)

func main() {
	switch *level {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.FatalLevel)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	_ = ctx

	wc, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create watcher")
	}
	defer func() {
		if err := wc.Close(); err != nil {
			log.Fatal().Err(err).Msg("failed to close watcher")
		}
	}()

	if err := wc.Add(*path); err != nil {
		log.Fatal().Err(err).Msg("failed to add watch")
	}

	if err := gitPull(*path); err != nil {
		log.Fatal().Err(err).Str("path", *path).Msg("failed to pull")
	}

	prevChangeTime := time.Now()
	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("exiting")
			return
		case event, ok := <-wc.Events:
			if !ok {
				log.Error().Any("event", event).Msg("watcher events channel closed")
				return
			}

			if time.Since(prevChangeTime) < 5*time.Second {
				log.Info().Msg("skip event")
				continue
			}

			prevChangeTime = time.Now()

			log.Info().Any("event", event).Str("name", event.Name).Msg("event received")

			if err := gitPull(*path); err != nil {
				log.Error().Err(err).Msg("failed to pull")
			}

			if err := gitAddAll(*path); err != nil {
				log.Error().Err(err).Msg("failed to add")
				continue
			}

			if err := gitCommit(*path, *device); err != nil {
				log.Error().Err(err).Msg("failed to commit")
				continue
			}

			if err := gitPush(*path); err != nil {
				log.Error().Err(err).Msg("failed to push")
				continue
			}
		case err, ok := <-wc.Errors:
			if !ok {
				log.Error().Msg("watcher errors channel closed")
				return
			}
			log.Error().Err(err).Msg("error received")
		}
	}
}

// Git Execution

func executeCommand(dir string, command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		log.Error().Err(err).Str("output", string(output)).Msg("failed to execute command")
		return err
	}

	log.Info().Str("output", string(output)).Msg("command executed")
	return nil
}

// git add .
func gitAddAll(path string) error {
	return executeCommand(path, "git", "add", ".")
}

// git commit -m "~"
func gitCommit(path string, deviceName string) error {
	return executeCommand(path, "git", "commit", "-m", fmt.Sprintf("auto sync from %s", deviceName))
}

// git push
func gitPush(path string) error {
	return executeCommand(path, "git", "push")
}

// git pull
func gitPull(path string) error {
	return executeCommand(path, "git", "pull")
}
