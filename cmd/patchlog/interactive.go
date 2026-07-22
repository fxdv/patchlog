package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fxdv/patchlog/pkg/config"
	"github.com/fxdv/patchlog/pkg/render"

	"gopkg.in/yaml.v3"
)

func cmdInit() {
	reader := bufio.NewReader(os.Stdin)
	cfg := config.Default()

	fmt.Println("patchlog init — interactive setup")
	fmt.Println(strings.Repeat("-", 40))

	fmt.Print("Repository (owner/name) [optional]: ")
	repo, _ := reader.ReadString('\n')
	repo = strings.TrimSpace(repo)
	if repo != "" {
		cfg.Repo = repo
	}

	fmt.Print("AI provider (ollama/openai/anthropic) [ollama]: ")
	aiProvider, _ := reader.ReadString('\n')
	aiProvider = strings.TrimSpace(aiProvider)
	if aiProvider == "" {
		aiProvider = "ollama"
	}
	cfg.AI.Provider = aiProvider

	if aiProvider == "openai" || aiProvider == "anthropic" {
		fmt.Print("API key (or $ENV_VAR): ")
		key, _ := reader.ReadString('\n')
		key = strings.TrimSpace(key)
		cfg.AI.APIKey = key
		if key != "" && !strings.HasPrefix(key, "$") {
			fmt.Println("  ⚠ Consider using $ENV_VAR instead of pasting secrets directly")
		}

		defaultModel := "gpt-4o-mini"
		if aiProvider == "anthropic" {
			defaultModel = "claude-3-haiku-20240307"
		}
		fmt.Printf("Model [%s]: ", defaultModel)
		model, _ := reader.ReadString('\n')
		model = strings.TrimSpace(model)
		if model != "" {
			cfg.AI.Model = model
		} else {
			cfg.AI.Model = defaultModel
		}
	}
	if aiProvider == "ollama" {
		fmt.Print("Ollama base URL [http://localhost:11434]: ")
		url, _ := reader.ReadString('\n')
		url = strings.TrimSpace(url)
		if url != "" {
			cfg.AI.BaseURL = url
		}
		fmt.Print("Model [llama3.2]: ")
		model, _ := reader.ReadString('\n')
		model = strings.TrimSpace(model)
		if model != "" {
			cfg.AI.Model = model
		}
	}

	fmt.Print("Git provider (github/gitlab/gitea) [none]: ")
	prov, _ := reader.ReadString('\n')
	prov = strings.TrimSpace(prov)
	if prov != "" {
		cfg.Provider.Type = prov
		fmt.Print("Token (or $ENV_VAR): ")
		tok, _ := reader.ReadString('\n')
		tok = strings.TrimSpace(tok)
		cfg.Provider.Token = tok
		if tok != "" && !strings.HasPrefix(tok, "$") {
			fmt.Println("  ⚠ Consider using $ENV_VAR instead of pasting secrets directly")
		}

		if prov == "gitea" {
			fmt.Print("Base URL (e.g. https://gitea.example.com): ")
			bu, _ := reader.ReadString('\n')
			bu = strings.TrimSpace(bu)
			cfg.Provider.BaseURL = bu
		}
		if repo != "" {
			cfg.Provider.Repo = repo
		}
	}

	fmt.Print("Jira base URL (e.g. https://yourorg.atlassian.net) [none]: ")
	jiraURL, _ := reader.ReadString('\n')
	jiraURL = strings.TrimSpace(jiraURL)
	if jiraURL != "" {
		cfg.Jira.BaseURL = jiraURL
		fmt.Print("Jira email: ")
		email, _ := reader.ReadString('\n')
		cfg.Jira.Email = strings.TrimSpace(email)
		fmt.Print("Jira API token (or $ENV_VAR): ")
		token, _ := reader.ReadString('\n')
		cfg.Jira.APIToken = strings.TrimSpace(token)
		if cfg.Jira.APIToken != "" && !strings.HasPrefix(cfg.Jira.APIToken, "$") {
			fmt.Println("  ⚠ Consider using $ENV_VAR instead of pasting secrets directly")
		}
		fmt.Print("Jira project key (e.g. PROJ) [optional]: ")
		key, _ := reader.ReadString('\n')
		cfg.Jira.ProjectKey = strings.TrimSpace(key)
	}

	fmt.Print("Publish to Confluence? (y/n) [n]: ")
	confChoice, _ := reader.ReadString('\n')
	confChoice = strings.TrimSpace(strings.ToLower(confChoice))
	if confChoice == "y" || confChoice == "yes" {
		if cfg.Jira.BaseURL != "" {
			cfg.Confluence.BaseURL = cfg.Jira.BaseURL
			cfg.Confluence.Email = cfg.Jira.Email
			cfg.Confluence.APIToken = cfg.Jira.APIToken
			fmt.Println("  Using Jira credentials for Confluence")
		} else {
			fmt.Print("Confluence base URL: ")
			cURL, _ := reader.ReadString('\n')
			cfg.Confluence.BaseURL = strings.TrimSpace(cURL)
			fmt.Print("Confluence email: ")
			cEmail, _ := reader.ReadString('\n')
			cfg.Confluence.Email = strings.TrimSpace(cEmail)
			fmt.Print("Confluence API token (or $ENV_VAR): ")
			cToken, _ := reader.ReadString('\n')
			cfg.Confluence.APIToken = strings.TrimSpace(cToken)
			if cfg.Confluence.APIToken != "" && !strings.HasPrefix(cfg.Confluence.APIToken, "$") {
				fmt.Println("  ⚠ Consider using $ENV_VAR instead of pasting secrets directly")
			}
		}
		fmt.Print("Confluence space key (e.g. ENG): ")
		space, _ := reader.ReadString('\n')
		cfg.Confluence.SpaceKey = strings.TrimSpace(space)
		fmt.Print("Confluence parent page ID [optional]: ")
		parentID, _ := reader.ReadString('\n')
		cfg.Confluence.ParentPageID = strings.TrimSpace(parentID)
		fmt.Print("Confluence labels (comma-separated, e.g. release-notes,frontend) [optional]: ")
		labelsInput, _ := reader.ReadString('\n')
		labelsInput = strings.TrimSpace(labelsInput)
		if labelsInput != "" {
			for _, l := range strings.Split(labelsInput, ",") {
				l = strings.TrimSpace(l)
				if l != "" {
					cfg.Confluence.Labels = append(cfg.Confluence.Labels, l)
				}
			}
		}
	}

	fmt.Print("Accumulate full changelog? (y/n) [n]: ")
	clChoice, _ := reader.ReadString('\n')
	clChoice = strings.TrimSpace(strings.ToLower(clChoice))
	if clChoice == "y" || clChoice == "yes" {
		cfg.Changelog.Accumulate = true
		fmt.Print("Destination (md/wiki/confluence) [md]: ")
		dest, _ := reader.ReadString('\n')
		dest = strings.TrimSpace(strings.ToLower(dest))
		if dest == "" {
			dest = "md"
		}
		cfg.Changelog.Destination = dest

		if dest == "md" {
			fmt.Print("Changelog file path [CHANGELOG.md]: ")
			filePath, _ := reader.ReadString('\n')
			filePath = strings.TrimSpace(filePath)
			if filePath != "" {
				cfg.Changelog.File = filePath
			}
		}
		if dest == "wiki" {
			fmt.Print("Wiki page title [Changelog]: ")
			wikiTitle, _ := reader.ReadString('\n')
			wikiTitle = strings.TrimSpace(wikiTitle)
			if wikiTitle != "" {
				cfg.Changelog.Title = wikiTitle
			}
			fmt.Print("Wiki page slug [changelog]: ")
			wikiSlug, _ := reader.ReadString('\n')
			wikiSlug = strings.TrimSpace(wikiSlug)
			if wikiSlug != "" {
				cfg.Changelog.Slug = wikiSlug
			}
		}
		if dest == "confluence" {
			fmt.Print("Confluence page title [Changelog]: ")
			confTitle, _ := reader.ReadString('\n')
			confTitle = strings.TrimSpace(confTitle)
			if confTitle != "" {
				cfg.Changelog.Title = confTitle
			}
		}

		fmt.Print("Prettify with emoji icons? (y/n) [y]: ")
		emojiChoice, _ := reader.ReadString('\n')
		emojiChoice = strings.TrimSpace(strings.ToLower(emojiChoice))
		emojis := true
		if emojiChoice == "n" || emojiChoice == "no" {
			emojis = false
		}
		cfg.Changelog.Emojis = &emojis
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating config: %v\n", err)
		os.Exit(1)
	}
	if err := atomicWriteFile("patchlog.yaml", data, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing patchlog.yaml: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("\nWritten patchlog.yaml")
	fmt.Println("⚠ Add patchlog.yaml to .gitignore to avoid committing secrets")
}

func cmdReview(output []byte) ([]byte, bool, error) {
	tmp, err := os.CreateTemp("", "patchlog-review-*.md")
	if err != nil {
		return nil, false, fmt.Errorf("create review file: %w", err)
	}
	tmpFile := tmp.Name()
	defer os.Remove(tmpFile)
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return nil, false, fmt.Errorf("secure review file: %w", err)
	}
	if _, err := tmp.Write(output); err != nil {
		_ = tmp.Close()
		return nil, false, fmt.Errorf("write review file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, false, fmt.Errorf("close review file: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Release notes written to %s\n", tmpFile)
	fmt.Fprintf(os.Stderr, "Edit the file, then press Enter to confirm or type 'abort' to cancel: ")
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)

	if line == "abort" {
		fmt.Fprintf(os.Stderr, "Cancelled.\n")
		return nil, false, nil
	}

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		return nil, false, fmt.Errorf("read review file: %w", err)
	}

	return data, true, nil
}

func cmdRecover(jsonPath string) {
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", jsonPath, err)
		os.Exit(1)
	}

	var report render.Report
	if err := json.Unmarshal(data, &report); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing: %v\n", err)
		os.Exit(1)
	}

	markdown, err := render.Markdown(report)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering: %v\n", err)
		os.Exit(1)
	}
	os.Stdout.Write(markdown)
}
