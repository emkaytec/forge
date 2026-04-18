package ui

import (
	"fmt"
	"io"
)

func Success(w io.Writer, message string) {
	writeSeverity(w, stylesFor(w).Success, IconSuccess, message)
}

func Warn(w io.Writer, message string) {
	writeSeverity(w, stylesFor(w).Warning, IconWarning, message)
}

func Error(w io.Writer, message string) {
	writeSeverity(w, stylesFor(w).Error, IconError, message)
}

func writeSeverity(w io.Writer, style interface{ Render(...string) string }, icon, message string) {
	fmt.Fprintln(w, style.Render(icon+" "+message))
}
