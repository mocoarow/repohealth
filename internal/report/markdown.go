package report

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mocoarow/repohealth/internal/snapshot"
	"github.com/mocoarow/repohealth/internal/source"
)

const (
	// MarkdownTop is how many entries of each ranked list the Markdown report shows.
	MarkdownTop = 10
	// MarkdownTrend is how many of the latest snapshots the Markdown report shows in its trends.
	MarkdownTrend = 24

	dash = "-"

	colArea = "Area"
	colFile = "File"
)

type mdTable struct {
	heading string
	header  []string
	rows    [][]string
}

// mdHeading is a heading with no table, which groups the tables after it.
func mdHeading(heading string) mdTable {
	return mdTable{heading: heading, header: nil, rows: nil}
}

// WriteMarkdown writes the Markdown report to path, creating its directory and overwriting the file.
func WriteMarkdown(path string, summaries []snapshot.Summary, latest snapshot.Detail) error {
	// render first, so a report that fails leaves the committed file as it was
	var b bytes.Buffer
	if err := Markdown(&b, summaries, latest); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}

	return writeFile(path, func(w io.Writer) error {
		if _, err := w.Write(b.Bytes()); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}

		return nil
	})
}

// Markdown writes a report of the history and the latest detail as GitHub-flavored Markdown.
func Markdown(w io.Writer, summaries []snapshot.Summary, latest snapshot.Detail) error {
	functions, err := mdFunctions(latest.Functions)
	if err != nil {
		return err
	}

	largest, err := mdLargest(latest.LargestFiles)
	if err != nil {
		return err
	}

	history := sortedSummaries(latestPerRev(summaries))

	var b strings.Builder

	writeTitle(&b, latest.Header)

	tables := slices.Concat(
		[]mdTable{{heading: "## Areas", header: areasHeader(), rows: mdAreas(history, latest.Rev)}},
		mdTrends(history),
		mdCrossArea(latest),
		mdDependencies(latest),
		[]mdTable{
			mdHotspots(latest),
			{heading: "## Functions", header: []string{"Lang", "Function", colFile, "Line", "Cognit", "Cyclo"}, rows: functions},
		},
		mdCoverage(latest),
		[]mdTable{{heading: "## Largest files", header: []string{colFile, colArea, "Lang", "Lines"}, rows: largest}},
	)

	for _, t := range tables {
		writeMDTable(&b, t)
	}

	if _, err := io.WriteString(w, strings.TrimRight(b.String(), "\n")+"\n"); err != nil {
		return fmt.Errorf("write markdown: %w", err)
	}

	return nil
}

func writeTitle(b *strings.Builder, h snapshot.Header) {
	tools := "repohealth " + h.Tools.Repohealth
	if h.Tools.Biome != "" {
		tools += ", Biome " + h.Tools.Biome
	}

	fmt.Fprintf(b, "# Repository health\n\nSnapshot of %s committed at %s, taken at %s (%s).\n\n",
		h.Rev, formatTime(h.CommittedAt), formatTime(h.TakenAt), tools)
}

func writeMDTable(b *strings.Builder, t mdTable) {
	b.WriteString(t.heading + "\n\n")

	if t.header == nil {
		return
	}

	if len(t.rows) == 0 {
		b.WriteString("None.\n\n")

		return
	}

	writeMDRow(b, t.header)

	seps := make([]string, len(t.header))
	for i := range seps {
		seps[i] = "---"
	}

	writeMDRow(b, seps)

	for _, r := range t.rows {
		writeMDRow(b, r)
	}

	b.WriteString("\n")
}

func writeMDRow(b *strings.Builder, cells []string) {
	b.WriteString("| " + strings.Join(cells, " | ") + " |\n")
}

// code writes s as a code span that a table cell keeps as typed: the fence is one backtick longer than any run in s.
func code(s string) string {
	if s == "" {
		return ""
	}

	s = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(s)

	longest, run := 0, 0
	for _, r := range s {
		if r == '`' {
			run++
			longest = max(longest, run)
		} else {
			run = 0
		}
	}

	if strings.HasPrefix(s, "`") || strings.HasSuffix(s, "`") {
		s = " " + s + " "
	}

	fence := strings.Repeat("`", longest+1)

	return fence + strings.ReplaceAll(s, "|", `\|`) + fence
}

func coverageCell(covered, lines int) string {
	pct, ok := percent(covered, lines)
	if !ok {
		return dash
	}

	return pct + "%"
}

func langText(l source.Lang) (string, error) {
	b, err := l.MarshalText()
	if err != nil {
		return "", fmt.Errorf("language: %w", err)
	}

	return string(b), nil
}
