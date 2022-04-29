package appcommon

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApp_Close(t *testing.T) {
	t.Run("single error", func(t *testing.T) {
		app := App{closers: []func() error{
			func() error { return errors.New("yikes") },
		}}

		err := app.Close()
		require.Error(t, err)
		require.Equal(t, "error 1: yikes", err.Error())
	})

	t.Run("multiple errors", func(t *testing.T) {
		app := App{closers: []func() error{
			func() error { return errors.New("yikes") },
			func() error { return errors.New("arghhhhh") },
		}}

		err := app.Close()
		require.Equal(t, "error 1: yikes, error 2: arghhhhh", err.Error())
	})

	t.Run("single failed close with multiple successful closes errors", func(t *testing.T) {
		app := App{closers: []func() error{
			func() error { return nil },
			func() error { return errors.New("arghhhhh") },
			func() error { return nil },
		}}

		err := app.Close()
		require.Error(t, err)
		require.Equal(t, "error 1: arghhhhh", err.Error())
	})

	t.Run("no closers no errors", func(t *testing.T) {
		app := App{}

		err := app.Close()
		require.NoError(t, err)
	})

	t.Run("empty closers no errors", func(t *testing.T) {
		app := App{closers: []func() error{}}
		err := app.Close()
		require.NoError(t, err)
	})

	t.Run("all closers successful no errors", func(t *testing.T) {
		app := App{closers: []func() error{
			func() error { return nil },
			func() error { return nil },
		}}

		err := app.Close()
		require.NoError(t, err)
	})

}
