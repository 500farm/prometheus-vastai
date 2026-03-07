package main

import (
	"fmt"
	"log/slog"
	"os"
)

type logger struct{}

var log logger

func (logger) Infoln(args ...any) {
	slog.Info(fmt.Sprint(args...))
}

func (logger) Infof(format string, args ...any) {
	slog.Info(fmt.Sprintf(format, args...))
}

func (logger) Warnln(args ...any) {
	slog.Warn(fmt.Sprint(args...))
}

func (logger) Errorln(args ...any) {
	slog.Error(fmt.Sprint(args...))
}

func (logger) Fatalln(args ...any) {
	slog.Error(fmt.Sprint(args...))
	os.Exit(1)
}

func (logger) Fatal(args ...any) {
	slog.Error(fmt.Sprint(args...))
	os.Exit(1)
}