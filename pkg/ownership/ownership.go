// Package ownership computes code ownership heatmaps from commit history per directory and file.
package ownership

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/fxdv/patchlog/pkg/commit"
	"github.com/fxdv/patchlog/pkg/gitlog"
	"github.com/fxdv/patchlog/pkg/safehtml"
)

type FileOwnership struct {
	Path    string
	Authors map[string]int
	Primary string
	Commits int
}

type DirectoryOwnership struct {
	Dir     string
	Authors map[string]int
	Files   int
	Primary string
}

func Compute(ctx context.Context, fetcher *gitlog.Fetcher, commits []commit.Commit) ([]FileOwnership, []DirectoryOwnership) {
	fileAuthors := make(map[string]map[string]int)
	dirAuthors := make(map[string]map[string]int)
	dirFiles := make(map[string]int)

	for _, c := range commits {
		files, err := fetcher.ChangedFiles(ctx, c.Hash)
		if err != nil {
			continue
		}
		for _, f := range files {
			if fileAuthors[f] == nil {
				fileAuthors[f] = make(map[string]int)
			}
			fileAuthors[f][c.Author]++

			dir := topDir(f)
			if dirAuthors[dir] == nil {
				dirAuthors[dir] = make(map[string]int)
			}
			dirAuthors[dir][c.Author]++
			dirFiles[dir]++
		}
	}

	var fileOwnerships []FileOwnership
	for path, authors := range fileAuthors {
		fo := FileOwnership{Path: path, Authors: authors}
		total := 0
		for _, c := range authors {
			total += c
		}
		fo.Commits = total
		fo.Primary = findPrimary(authors)
		fileOwnerships = append(fileOwnerships, fo)
	}
	sort.Slice(fileOwnerships, func(i, j int) bool {
		return fileOwnerships[i].Commits > fileOwnerships[j].Commits
	})
	if len(fileOwnerships) > 50 {
		fileOwnerships = fileOwnerships[:50]
	}

	var dirOwnerships []DirectoryOwnership
	for dir, authors := range dirAuthors {
		do := DirectoryOwnership{Dir: dir, Authors: authors, Files: dirFiles[dir]}
		do.Primary = findPrimary(authors)
		dirOwnerships = append(dirOwnerships, do)
	}
	sort.Slice(dirOwnerships, func(i, j int) bool {
		return dirOwnerships[i].Files > dirOwnerships[j].Files
	})

	return fileOwnerships, dirOwnerships
}

func findPrimary(authors map[string]int) string {
	max := 0
	primary := ""
	for name, count := range authors {
		if count > max {
			max = count
			primary = name
		}
	}
	return primary
}

func topDir(path string) string {
	if idx := strings.Index(path, "/"); idx >= 0 {
		return path[:idx]
	}
	return "."
}

func FormatHTML(files []FileOwnership, dirs []DirectoryOwnership) string {
	if len(dirs) == 0 {
		return ""
	}
	var sb strings.Builder

	allAuthors := make(map[string]bool)
	for _, d := range dirs {
		for name := range d.Authors {
			allAuthors[name] = true
		}
	}
	authorList := make([]string, 0, len(allAuthors))
	for name := range allAuthors {
		authorList = append(authorList, name)
	}
	sort.Strings(authorList)

	sb.WriteString(`<div class="table-wrap"><table><thead><tr><th>Directory</th><th>Files</th>`)
	for _, name := range authorList {
		short := name
		if len(short) > 10 {
			short = short[:8] + "…"
		}
		fmt.Fprintf(&sb, `<th>%s</th>`, safehtml.Text(short))
	}
	sb.WriteString(`</tr></thead><tbody>`)

	for _, d := range dirs {
		sb.WriteString(`<tr>`)
		fmt.Fprintf(&sb, `<td style="font-family: var(--mono); font-size: 11px;">%s</td>`, safehtml.Text(d.Dir))
		fmt.Fprintf(&sb, `<td>%d</td>`, d.Files)
		total := 0
		for _, c := range d.Authors {
			total += c
		}
		for _, name := range authorList {
			count := d.Authors[name]
			if count == 0 {
				sb.WriteString(`<td style="color: var(--text-dim);">·</td>`)
			} else {
				pct := float64(count) / float64(total) * 100
				intensity := int(pct / 10)
				if intensity > 9 {
					intensity = 9
				}
				cls := fmt.Sprintf("cell-heat-%d", intensity)
				fmt.Fprintf(&sb, `<td class="%s" style="text-align: center;">%d</td>`, cls, count)
			}
		}
		sb.WriteString(`</tr>`)
	}
	sb.WriteString(`</tbody></table></div>`)

	sb.WriteString(`<style>`)
	for i := 0; i <= 9; i++ {
		opacity := 0.08 + float64(i)*0.09
		fmt.Fprintf(&sb, `.cell-heat-%d { background: rgba(79,70,229,%.2f); }`, i, opacity)
	}
	sb.WriteString(`</style>`)

	return sb.String()
}
