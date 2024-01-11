package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComicsPage(t *testing.T) {
	testCases := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "Valid URL",
			args:    []string{"", "https://drawnstories.ru/comics/Oni-press/rick-and-morty"},
			wantErr: false,
		},
		{
			name:    "No URL",
			args:    []string{""},
			wantErr: true,
		},
		{
			name:    "Invalid URL",
			args:    []string{"", "invalid_url"},
			wantErr: true,
		},
		{
			name:    "Unsupported Site",
			args:    []string{"", "https://unsupported.com/comics/Oni-press/rick-and-morty"},
			wantErr: true,
		},
		{
			name:    "Non-Comic Page",
			args:    []string{"", "https://drawnstories.ru/non-comic-page"},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := comicsPage(tc.args)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
