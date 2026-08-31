package cli

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Hyaxia/blogwatcher/internal/model"
	"github.com/Hyaxia/blogwatcher/internal/scanner"
	"github.com/fatih/color"
)

func TestResolveScanOptionsDefaults(t *testing.T) {
	clearMetadataEnvironment(t)
	options, err := scanOptionsForArgs()
	if err != nil {
		t.Fatalf("resolve defaults: %v", err)
	}
	want := scanner.Options{
		StoreDescriptions:   true,
		StoreKeywords:       false,
		DescriptionMaxChars: defaultDescriptionMax,
	}
	if options != want {
		t.Fatalf("expected %+v, got %+v", want, options)
	}
}

func TestResolveScanOptionsFromEnvironment(t *testing.T) {
	tests := []struct {
		name         string
		descriptions string
		keywords     string
		maximum      string
		want         scanner.Options
	}{
		{name: "documented booleans", descriptions: "true", keywords: "false", maximum: "250", want: scanner.Options{StoreDescriptions: true, DescriptionMaxChars: 250}},
		{name: "standard true forms", descriptions: "1", keywords: "TRUE", want: scanner.Options{StoreDescriptions: true, StoreKeywords: true, DescriptionMaxChars: 1000}},
		{name: "standard false forms", descriptions: "0", keywords: "FALSE", want: scanner.Options{DescriptionMaxChars: 1000}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearMetadataEnvironment(t)
			t.Setenv(storeDescriptionsEnv, test.descriptions)
			t.Setenv(storeKeywordsEnv, test.keywords)
			if test.maximum != "" {
				t.Setenv(descriptionMaxCharsEnv, test.maximum)
			}
			got, err := scanOptionsForArgs()
			if err != nil {
				t.Fatalf("resolve environment: %v", err)
			}
			if got != test.want {
				t.Fatalf("expected %+v, got %+v", test.want, got)
			}
		})
	}
}

func TestResolveScanOptionsFlagsOverrideEnvironment(t *testing.T) {
	clearMetadataEnvironment(t)
	t.Setenv(storeDescriptionsEnv, "true")
	t.Setenv(storeKeywordsEnv, "true")
	t.Setenv(descriptionMaxCharsEnv, "250")

	options, err := scanOptionsForArgs("--store-descriptions=false", "--store-keywords=false")
	if err == nil || !strings.Contains(err.Error(), "requires description storage") {
		t.Fatalf("expected environment limit to conflict with explicit disable, got %+v, %v", options, err)
	}

	t.Setenv(descriptionMaxCharsEnv, "")
	options, err = scanOptionsForArgs("--store-descriptions=false", "--store-keywords=false")
	if err != nil {
		t.Fatalf("resolve flag precedence: %v", err)
	}
	if options.StoreDescriptions || options.StoreKeywords {
		t.Fatalf("explicit false flags did not override environment: %+v", options)
	}

	t.Setenv(storeDescriptionsEnv, "true")
	t.Setenv(descriptionMaxCharsEnv, "250")
	options, err = scanOptionsForArgs("--description-max-chars=0")
	if err != nil {
		t.Fatalf("resolve unlimited flag: %v", err)
	}
	if !options.StoreDescriptions || options.DescriptionMaxChars != 0 {
		t.Fatalf("flag did not override environment maximum: %+v", options)
	}
}

func TestResolveScanOptionsRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		args []string
		want string
	}{
		{name: "invalid description boolean", env: map[string]string{storeDescriptionsEnv: "sometimes"}, want: "must be true or false"},
		{name: "invalid keyword boolean", env: map[string]string{storeKeywordsEnv: "sometimes"}, want: "must be true or false"},
		{name: "limit flag when disabled", args: []string{"--store-descriptions=false", "--description-max-chars=20"}, want: "requires description storage"},
		{name: "limit environment when disabled", env: map[string]string{storeDescriptionsEnv: "false", descriptionMaxCharsEnv: "20"}, want: "requires description storage"},
		{name: "invalid limit", env: map[string]string{storeDescriptionsEnv: "true", descriptionMaxCharsEnv: "many"}, want: "must be an integer"},
		{name: "negative limit", args: []string{"--store-descriptions", "--description-max-chars=-1"}, want: "zero or greater"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearMetadataEnvironment(t)
			for key, value := range test.env {
				t.Setenv(key, value)
			}
			_, err := scanOptionsForArgs(test.args...)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected error containing %q, got %v", test.want, err)
			}
		})
	}
}

func TestPrintArticleDefaultOutputIsUnchanged(t *testing.T) {
	published := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	article := model.Article{
		ID: 7, Title: "Title", URL: "https://example.com/article", PublishedDate: &published,
		Description: "Stored metadata must stay hidden.", Keywords: "one,two",
	}
	got := captureStdout(t, func() { printArticle(article, "Example", false, false) })
	want := "  [7] [new] Title\n" +
		"       Blog: Example\n" +
		"       URL: https://example.com/article\n" +
		"       Published: 2026-08-25\n\n"
	if got != want {
		t.Fatalf("default output changed\nwant:\n%q\ngot:\n%q", want, got)
	}
}

func TestPrintArticleMetadataFlagsAreIndependent(t *testing.T) {
	description := strings.Repeat("complete description ", 8)
	article := model.Article{ID: 1, Title: "Title", URL: "https://example.com/1", Description: description, Keywords: "one,two"}

	descriptionOnly := captureStdout(t, func() { printArticle(article, "Example", true, false) })
	if !strings.Contains(descriptionOnly, "Description: "+description) || strings.Contains(descriptionOnly, "Keywords:") {
		t.Fatalf("unexpected description-only output: %q", descriptionOnly)
	}
	if !strings.Contains(descriptionOnly, strings.Repeat("complete description ", 6)) {
		t.Fatalf("stored description was unexpectedly preview-truncated: %q", descriptionOnly)
	}

	keywordsOnly := captureStdout(t, func() { printArticle(article, "Example", false, true) })
	if !strings.Contains(keywordsOnly, "Keywords: one,two") || strings.Contains(keywordsOnly, "Description:") {
		t.Fatalf("unexpected keywords-only output: %q", keywordsOnly)
	}

	both := captureStdout(t, func() { printArticle(article, "Example", true, true) })
	if !strings.Contains(both, "Keywords: one,two") || !strings.Contains(both, "Description: "+description) {
		t.Fatalf("unexpected combined output: %q", both)
	}
}

func scanOptionsForArgs(args ...string) (scanner.Options, error) {
	cmd := newScanCommand()
	if err := cmd.ParseFlags(args); err != nil {
		return scanner.Options{}, err
	}
	descriptions, _ := cmd.Flags().GetBool("store-descriptions")
	keywords, _ := cmd.Flags().GetBool("store-keywords")
	maximum, _ := cmd.Flags().GetInt("description-max-chars")
	return resolveScanOptions(cmd, descriptions, keywords, maximum)
}

func clearMetadataEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv(storeDescriptionsEnv, "")
	t.Setenv(storeKeywordsEnv, "")
	t.Setenv(descriptionMaxCharsEnv, "")
}

func captureStdout(t *testing.T, action func()) string {
	t.Helper()
	previousStdout := os.Stdout
	previousNoColor := color.NoColor
	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = writeEnd
	color.NoColor = true
	action()
	_ = writeEnd.Close()
	os.Stdout = previousStdout
	color.NoColor = previousNoColor
	output, err := io.ReadAll(readEnd)
	_ = readEnd.Close()
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return string(output)
}
