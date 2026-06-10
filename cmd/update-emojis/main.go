package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	emojiFile = flag.String("emoji-file", "dev-emojis.txt", "path to dev emoji mapping file")
	yamlDir   = flag.String("yaml-dir", "yaml", "directory containing yaml files to update")
	dryRun    = flag.Bool("dry-run", false, "print changes without writing files")
)

var (
	devEmojiLinePattern = regexp.MustCompile(`^\s*([^:#\s]+)\s*:\s*(<a?:[^:>\s]+:\d+>)\s*$`)
	customEmojiPattern  = regexp.MustCompile(`<a?:([^:>\s]+):\d+>`)
)

func main() {
	flag.Parse()

	emojis, err := readDevEmojis(*emojiFile)
	if err != nil {
		fatalf("failed to read emoji file: %v", err)
	}

	if len(emojis) == 0 {
		fatalf("no emojis found in %s", *emojiFile)
	}

	changedFiles := 0
	replacements := 0

	err = filepath.WalkDir(*yamlDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !isYAMLFile(path) {
			return nil
		}

		fileChanged, fileReplacements, err := updateFile(path, emojis, *dryRun)
		if err != nil {
			return err
		}

		if fileChanged {
			changedFiles++
			replacements += fileReplacements
		}

		return nil
	})
	if err != nil {
		fatalf("failed to update yaml files: %v", err)
	}

	if *dryRun {
		fmt.Printf("dry run complete: %d files would change, %d replacements would be made\n", changedFiles, replacements)
		return
	}

	fmt.Printf("updated %d files with %d replacements\n", changedFiles, replacements)
}

func readDevEmojis(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	emojis := make(map[string]string)

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		matches := devEmojiLinePattern.FindStringSubmatch(line)
		if matches == nil {
			return nil, fmt.Errorf("%s:%d: invalid emoji mapping line %q", path, lineNumber, line)
		}

		name := matches[1]
		mention := matches[2]
		emojis[name] = mention
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return emojis, nil
}

func updateFile(path string, emojis map[string]string, dryRun bool) (bool, int, error) {
	original, err := os.ReadFile(path)
	if err != nil {
		return false, 0, err
	}

	replacements := 0
	updated := customEmojiPattern.ReplaceAllFunc(original, func(match []byte) []byte {
		parts := customEmojiPattern.FindSubmatch(match)
		if len(parts) != 2 {
			return match
		}

		name := string(parts[1])
		replacement, ok := emojis[name]
		if !ok {
			return match
		}

		if bytes.Equal(match, []byte(replacement)) {
			return match
		}

		replacements++
		return []byte(replacement)
	})

	if bytes.Equal(original, updated) {
		return false, 0, nil
	}

	if dryRun {
		fmt.Printf("would update %s with %d replacements\n", path, replacements)
		return true, replacements, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return false, 0, err
	}

	if err := os.WriteFile(path, updated, info.Mode()); err != nil {
		return false, 0, err
	}

	fmt.Printf("updated %s with %d replacements\n", path, replacements)
	return true, replacements, nil
}

func isYAMLFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
