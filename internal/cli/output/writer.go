package output

import (
	"fmt"
	"io"
	"os"
	"strings"
)

type writerFormatter struct {
	Formatter
	writer io.Writer
}

func WithWriter(formatter Formatter, writer io.Writer) Formatter {
	return &writerFormatter{Formatter: formatter, writer: writer}
}
func WriterOf(formatter Formatter) io.Writer {
	if f, ok := formatter.(*writerFormatter); ok {
		return f.writer
	}
	return os.Stdout
}
func Print(formatter Formatter, result string) error {
	_, err := fmt.Fprintln(WriterOf(formatter), strings.TrimRight(formatter.Format(result), "\n"))
	return err
}
