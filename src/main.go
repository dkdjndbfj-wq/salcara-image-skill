package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAPIURL  = "https://salcara.top"
	defaultModel   = "gpt-image-2.5-sunburst"
	defaultQuality = "max"
	maxBodyBytes   = 96 << 20
	maxImageBytes  = 64 << 20
)

var secretPattern = regexp.MustCompile(`(?i)(?:bearer\s+)?sk-[a-z0-9_-]{8,}`)

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

type apiClient struct {
	baseURL     string
	key         string
	httpClient  *http.Client
	maxAttempts int
}

type savedConfig struct {
	APIURL         string `json:"api_url"`
	APIKey         string `json:"api_key"`
	DefaultModel   string `json:"default_model"`
	DefaultQuality string `json:"default_quality"`
}

type imageDatum struct {
	B64JSON       string `json:"b64_json"`
	URL           string `json:"url"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

type imageResponse struct {
	Created      int64           `json:"created,omitempty"`
	Data         []imageDatum    `json:"data"`
	Usage        json.RawMessage `json:"usage,omitempty"`
	Size         string          `json:"size,omitempty"`
	Quality      string          `json:"quality,omitempty"`
	OutputFormat string          `json:"output_format,omitempty"`
}

type outputSummary struct {
	OK           bool            `json:"ok"`
	Command      string          `json:"command"`
	Model        string          `json:"model,omitempty"`
	Size         string          `json:"size,omitempty"`
	Quality      string          `json:"quality,omitempty"`
	Files        []string        `json:"files,omitempty"`
	Usage        json.RawMessage `json:"usage,omitempty"`
	RequestCount int             `json:"request_count,omitempty"`
	DryRun       bool            `json:"dry_run,omitempty"`
}

type generateOptions struct {
	Prompt            string `json:"prompt"`
	Model             string `json:"model,omitempty"`
	Size              string `json:"size,omitempty"`
	Quality           string `json:"quality,omitempty"`
	N                 int    `json:"n,omitempty"`
	Background        string `json:"background,omitempty"`
	OutputFormat      string `json:"output_format,omitempty"`
	OutputCompression int    `json:"output_compression,omitempty"`
	Moderation        string `json:"moderation,omitempty"`
	Out               string `json:"-"`
	OutDir            string `json:"-"`
	Force             bool   `json:"-"`
	DryRun            bool   `json:"-"`
}

type batchJob struct {
	Prompt            string `json:"prompt"`
	Model             string `json:"model,omitempty"`
	Size              string `json:"size,omitempty"`
	Quality           string `json:"quality,omitempty"`
	N                 int    `json:"n,omitempty"`
	Background        string `json:"background,omitempty"`
	OutputFormat      string `json:"output_format,omitempty"`
	OutputCompression *int   `json:"output_compression,omitempty"`
	Moderation        string `json:"moderation,omitempty"`
	Out               string `json:"out,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "configure":
		err = runConfigure(os.Args[2:])
	case "models":
		err = runModels(os.Args[2:])
	case "generate":
		err = runGenerate(os.Args[2:])
	case "edit":
		err = runEdit(os.Args[2:])
	case "generate-batch":
		err = runBatch(os.Args[2:])
	case "help", "-h", "--help":
		usage()
		return
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", sanitize(err.Error()))
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `Salcara Image CLI

Usage:
	  salcara-image configure --api-url URL --key-stdin [--default-model MODEL] [--default-quality QUALITY]
	  salcara-image models [--timeout 120s]
  salcara-image generate --prompt TEXT [options]
  salcara-image edit --image PATH --prompt TEXT [options]
  salcara-image generate-batch --file jobs.jsonl [options]

Environment variables override the user config file. SALCARA_API_URL defaults to https://salcara.top.
Use "salcara-image <command> --help" for command flags.`)
}

func newClient(timeout time.Duration, attempts int, dryRun bool) (*apiClient, error) {
	config, _ := loadConfig()
	rawURL := strings.TrimSpace(os.Getenv("SALCARA_API_URL"))
	if rawURL == "" {
		rawURL = strings.TrimSpace(config.APIURL)
		if rawURL == "" {
			rawURL = defaultAPIURL
		}
	}
	base, err := normalizeBaseURL(rawURL)
	if err != nil {
		return nil, err
	}
	key := strings.TrimSpace(os.Getenv("SALCARA_API_KEY"))
	if key == "" {
		key = strings.TrimSpace(config.APIKey)
	}
	if key == "" && !dryRun {
		return nil, errors.New("no API key is configured; run the bundled setup script")
	}
	if attempts < 1 || attempts > 3 {
		return nil, errors.New("max-attempts must be between 1 and 3")
	}
	return &apiClient{
		baseURL:     base,
		key:         key,
		httpClient:  &http.Client{Timeout: timeout},
		maxAttempts: attempts,
	}, nil
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "salcara-image", "config.json"), nil
}

func loadConfig() (savedConfig, error) {
	var config savedConfig
	path, err := configPath()
	if err != nil {
		return config, err
	}
	body, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return config, nil
	}
	if err != nil {
		return config, err
	}
	if err := json.Unmarshal(body, &config); err != nil {
		return config, fmt.Errorf("invalid config file: %w", err)
	}
	return config, nil
}

func configuredDefaultModel() string {
	config, err := loadConfig()
	if err == nil && strings.TrimSpace(config.DefaultModel) != "" {
		return config.DefaultModel
	}
	return defaultModel
}

func configuredDefaultQuality() string {
	config, err := loadConfig()
	if err == nil && oneOf(strings.TrimSpace(config.DefaultQuality), "auto", "low", "medium", "high", "xhigh", "max") {
		return strings.TrimSpace(config.DefaultQuality)
	}
	return defaultQuality
}

func runConfigure(args []string) error {
	fs := flag.NewFlagSet("configure", flag.ContinueOnError)
	apiURL := fs.String("api-url", "", "OpenAI-compatible API root or /v1 URL")
	defaultModelFlag := fs.String("default-model", "", "default image model")
	defaultQualityFlag := fs.String("default-quality", "", "default image quality: auto|low|medium|high|xhigh|max")
	keyStdin := fs.Bool("key-stdin", false, "read the API key from standard input")
	show := fs.Bool("show", false, "show non-secret configuration")
	if err := fs.Parse(args); err != nil {
		return err
	}
	config, err := loadConfig()
	if err != nil {
		return err
	}
	path, err := configPath()
	if err != nil {
		return err
	}
	if *show {
		return writeJSON(map[string]any{
			"api_url": coalesce(config.APIURL, defaultAPIURL), "default_model": coalesce(config.DefaultModel, defaultModel),
			"default_quality":    coalesce(config.DefaultQuality, defaultQuality),
			"api_key_configured": strings.TrimSpace(config.APIKey) != "", "config_file": path,
		})
	}
	if strings.TrimSpace(*apiURL) != "" {
		if _, err := normalizeBaseURL(*apiURL); err != nil {
			return err
		}
		config.APIURL = strings.TrimRight(strings.TrimSpace(*apiURL), "/")
	}
	if strings.TrimSpace(*defaultModelFlag) != "" {
		config.DefaultModel = strings.TrimSpace(*defaultModelFlag)
	}
	if strings.TrimSpace(*defaultQualityFlag) != "" {
		quality := strings.TrimSpace(*defaultQualityFlag)
		if !oneOf(quality, "auto", "low", "medium", "high", "xhigh", "max") {
			return errors.New("default-quality must be auto, low, medium, high, xhigh, or max")
		}
		config.DefaultQuality = quality
	}
	if *keyStdin {
		limited, err := io.ReadAll(io.LimitReader(os.Stdin, 16*1024))
		if err != nil {
			return err
		}
		config.APIKey = strings.TrimSpace(string(limited))
	}
	if config.APIURL == "" {
		config.APIURL = defaultAPIURL
	}
	if config.DefaultModel == "" {
		config.DefaultModel = defaultModel
	}
	if config.DefaultQuality == "" {
		config.DefaultQuality = defaultQuality
	}
	if config.APIKey == "" {
		return errors.New("API key is empty; pass it securely through --key-stdin")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	body, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return err
	}
	_ = os.Chmod(path, 0o600)
	return writeJSON(map[string]any{
		"ok": true, "api_url": config.APIURL, "default_model": config.DefaultModel,
		"default_quality":    config.DefaultQuality,
		"api_key_configured": true, "config_file": path,
	})
}

func normalizeBaseURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimRight(raw, "/"))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", errors.New("SALCARA_API_URL must be an absolute http(s) URL")
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", errors.New("SALCARA_API_URL must use http or https")
	}
	p := strings.TrimRight(u.Path, "/")
	if !strings.HasSuffix(p, "/v1") {
		p += "/v1"
	}
	u.Path = p
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/"), nil
}

func runModels(args []string) error {
	fs := flag.NewFlagSet("models", flag.ContinueOnError)
	timeout := fs.Duration("timeout", 120*time.Second, "request timeout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	client, err := newClient(*timeout, 1, false)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodGet, client.baseURL+"/models", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+client.key)
	body, status, _, err := client.do(req)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return apiStatusError(status, body, "")
	}
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return fmt.Errorf("invalid models response: %w", err)
	}
	return writeJSON(value)
}

func runGenerate(args []string) error {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	opts, promptFile, timeout, attempts := bindGenerateFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	prompt, err := readPrompt(opts.Prompt, *promptFile)
	if err != nil {
		return err
	}
	opts.Prompt = prompt
	if err := validateGenerate(opts); err != nil {
		return err
	}
	client, err := newClient(*timeout, *attempts, opts.DryRun)
	if err != nil {
		return err
	}
	if opts.DryRun {
		return writeJSON(map[string]any{
			"ok": true, "dry_run": true, "command": "generate", "endpoint": client.baseURL + "/images/generations",
			"model": opts.Model, "size": opts.Size, "quality": opts.Quality, "n": opts.N,
		})
	}
	resp, err := generate(client, opts)
	if err != nil {
		return err
	}
	files, err := saveImages(client, resp.Data, opts.Out, opts.OutDir, opts.OutputFormat, opts.Force)
	if err != nil {
		return err
	}
	return writeJSON(outputSummary{OK: true, Command: "generate", Model: opts.Model, Size: opts.Size, Quality: opts.Quality, Files: files, Usage: resp.Usage, RequestCount: 1})
}

func bindGenerateFlags(fs *flag.FlagSet) (*generateOptions, *string, *time.Duration, *int) {
	opts := &generateOptions{Model: configuredDefaultModel(), Size: "1024x1024", Quality: configuredDefaultQuality(), N: 1, Background: "auto", OutputFormat: "png", OutputCompression: -1, Moderation: "auto"}
	fs.StringVar(&opts.Prompt, "prompt", "", "image prompt")
	promptFile := fs.String("prompt-file", "", "UTF-8 prompt file")
	fs.StringVar(&opts.Model, "model", opts.Model, "model ID")
	fs.StringVar(&opts.Size, "size", opts.Size, "image size")
	fs.StringVar(&opts.Quality, "quality", opts.Quality, "auto|low|medium|high|xhigh|max")
	fs.IntVar(&opts.N, "n", opts.N, "number of images")
	fs.StringVar(&opts.Background, "background", opts.Background, "auto|opaque|transparent")
	fs.StringVar(&opts.OutputFormat, "output-format", opts.OutputFormat, "png|jpeg|webp")
	fs.IntVar(&opts.OutputCompression, "output-compression", opts.OutputCompression, "JPEG/WebP compression 0..100")
	fs.StringVar(&opts.Moderation, "moderation", opts.Moderation, "auto|low")
	fs.StringVar(&opts.Out, "out", "", "output file path")
	fs.StringVar(&opts.OutDir, "out-dir", ".", "output directory")
	fs.BoolVar(&opts.Force, "force", false, "replace existing output files")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "validate without a network request")
	timeout := fs.Duration("timeout", 5*time.Minute, "request timeout")
	attempts := fs.Int("max-attempts", 1, "request attempts, 1..3")
	return opts, promptFile, timeout, attempts
}

func readPrompt(direct, file string) (string, error) {
	if (strings.TrimSpace(direct) == "") == (strings.TrimSpace(file) == "") {
		return "", errors.New("provide exactly one of --prompt or --prompt-file")
	}
	if file == "" {
		return strings.TrimSpace(direct), nil
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("read prompt file: %w", err)
	}
	prompt := strings.TrimSpace(string(b))
	if prompt == "" {
		return "", errors.New("prompt file is empty")
	}
	return prompt, nil
}

func validateGenerate(o *generateOptions) error {
	if strings.TrimSpace(o.Prompt) == "" {
		return errors.New("prompt is required")
	}
	if len([]rune(o.Prompt)) > 32000 {
		return errors.New("prompt exceeds 32,000 characters")
	}
	if strings.TrimSpace(o.Model) == "" {
		return errors.New("model is required")
	}
	if o.N < 1 || o.N > 10 {
		return errors.New("n must be between 1 and 10")
	}
	if !oneOf(o.Quality, "auto", "low", "medium", "high", "xhigh", "max") {
		return errors.New("unsupported quality")
	}
	if !oneOf(o.Background, "auto", "opaque", "transparent") {
		return errors.New("unsupported background")
	}
	if !oneOf(o.OutputFormat, "png", "jpeg", "webp") {
		return errors.New("unsupported output-format")
	}
	if !oneOf(o.Moderation, "auto", "low") {
		return errors.New("unsupported moderation")
	}
	if o.OutputCompression != -1 {
		if o.OutputCompression < 0 || o.OutputCompression > 100 {
			return errors.New("output-compression must be between 0 and 100")
		}
		if o.OutputFormat == "png" {
			return errors.New("output-compression is available only for jpeg or webp")
		}
	}
	if o.Background == "transparent" && o.OutputFormat == "jpeg" {
		return errors.New("transparent background requires png or webp")
	}
	return validateSize(o.Size)
}

func validateSize(size string) error {
	if size == "auto" {
		return nil
	}
	parts := strings.Split(strings.ToLower(size), "x")
	if len(parts) != 2 {
		return errors.New("size must be WIDTHxHEIGHT or auto")
	}
	w, e1 := strconv.Atoi(parts[0])
	h, e2 := strconv.Atoi(parts[1])
	if e1 != nil || e2 != nil || w <= 0 || h <= 0 {
		return errors.New("size must contain positive integers")
	}
	return nil
}

func generate(client *apiClient, opts *generateOptions) (*imageResponse, error) {
	payload := map[string]any{
		"prompt": opts.Prompt, "model": opts.Model, "size": opts.Size, "quality": opts.Quality,
		"n": opts.N, "background": opts.Background, "output_format": opts.OutputFormat, "moderation": opts.Moderation,
	}
	if opts.OutputCompression >= 0 {
		payload["output_compression"] = opts.OutputCompression
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	respBody, err := client.requestWithRetry(func() (*http.Request, error) {
		req, e := http.NewRequest(http.MethodPost, client.baseURL+"/images/generations", bytes.NewReader(body))
		if e == nil {
			req.Header.Set("Content-Type", "application/json")
		}
		return req, e
	})
	if err != nil {
		return nil, err
	}
	return parseImageResponse(respBody)
}

func runEdit(args []string) error {
	fs := flag.NewFlagSet("edit", flag.ContinueOnError)
	var images stringList
	var prompt, promptFile, mask, model, size, quality, background, format, moderation, out, outDir string
	var n, compression, attempts int
	var force, dryRun bool
	var timeout time.Duration
	fs.Var(&images, "image", "input image; repeatable")
	fs.StringVar(&prompt, "prompt", "", "edit prompt")
	fs.StringVar(&promptFile, "prompt-file", "", "UTF-8 prompt file")
	fs.StringVar(&mask, "mask", "", "optional mask image")
	fs.StringVar(&model, "model", configuredDefaultModel(), "model ID")
	fs.StringVar(&size, "size", "auto", "image size")
	fs.StringVar(&quality, "quality", configuredDefaultQuality(), "auto|low|medium|high|xhigh|max")
	fs.IntVar(&n, "n", 1, "number of images")
	fs.StringVar(&background, "background", "auto", "auto|opaque|transparent")
	fs.StringVar(&format, "output-format", "png", "png|jpeg|webp")
	fs.IntVar(&compression, "output-compression", -1, "JPEG/WebP compression 0..100")
	fs.StringVar(&moderation, "moderation", "auto", "auto|low")
	fs.StringVar(&out, "out", "", "output file path")
	fs.StringVar(&outDir, "out-dir", ".", "output directory")
	fs.BoolVar(&force, "force", false, "replace existing output files")
	fs.BoolVar(&dryRun, "dry-run", false, "validate without a network request")
	fs.DurationVar(&timeout, "timeout", 5*time.Minute, "request timeout")
	fs.IntVar(&attempts, "max-attempts", 1, "request attempts, 1..3")
	if err := fs.Parse(args); err != nil {
		return err
	}
	resolvedPrompt, err := readPrompt(prompt, promptFile)
	if err != nil {
		return err
	}
	opts := &generateOptions{Prompt: resolvedPrompt, Model: model, Size: size, Quality: quality, N: n, Background: background, OutputFormat: format, OutputCompression: compression, Moderation: moderation, Out: out, OutDir: outDir, Force: force, DryRun: dryRun}
	if err := validateGenerate(opts); err != nil {
		return err
	}
	if len(images) == 0 {
		return errors.New("at least one --image is required")
	}
	for _, path := range append([]string(images), mask) {
		if path == "" {
			continue
		}
		info, e := os.Stat(path)
		if e != nil {
			return fmt.Errorf("input %q: %w", path, e)
		}
		if info.IsDir() {
			return fmt.Errorf("input %q is a directory", path)
		}
	}
	client, err := newClient(timeout, attempts, dryRun)
	if err != nil {
		return err
	}
	if dryRun {
		return writeJSON(map[string]any{
			"ok": true, "dry_run": true, "command": "edit", "endpoint": client.baseURL + "/images/edits",
			"model": model, "size": size, "quality": quality, "n": n, "input_count": len(images), "has_mask": mask != "",
		})
	}
	respBody, err := client.requestWithRetry(func() (*http.Request, error) {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		fields := map[string]string{"prompt": resolvedPrompt, "model": model, "size": size, "quality": quality, "n": strconv.Itoa(n), "background": background, "output_format": format, "moderation": moderation}
		if compression >= 0 {
			fields["output_compression"] = strconv.Itoa(compression)
		}
		for k, v := range fields {
			if e := writer.WriteField(k, v); e != nil {
				return nil, e
			}
		}
		for _, path := range images {
			if e := addFilePart(writer, "image", path); e != nil {
				return nil, e
			}
		}
		if mask != "" {
			if e := addFilePart(writer, "mask", mask); e != nil {
				return nil, e
			}
		}
		if e := writer.Close(); e != nil {
			return nil, e
		}
		req, e := http.NewRequest(http.MethodPost, client.baseURL+"/images/edits", &body)
		if e == nil {
			req.Header.Set("Content-Type", writer.FormDataContentType())
		}
		return req, e
	})
	if err != nil {
		return err
	}
	resp, err := parseImageResponse(respBody)
	if err != nil {
		return err
	}
	files, err := saveImages(client, resp.Data, out, outDir, format, force)
	if err != nil {
		return err
	}
	return writeJSON(outputSummary{OK: true, Command: "edit", Model: model, Size: size, Quality: quality, Files: files, Usage: resp.Usage, RequestCount: 1})
}

func addFilePart(writer *multipart.Writer, field, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	part, err := writer.CreateFormFile(field, filepath.Base(path))
	if err != nil {
		return err
	}
	_, err = io.Copy(part, io.LimitReader(f, maxImageBytes+1))
	return err
}

func runBatch(args []string) error {
	fs := flag.NewFlagSet("generate-batch", flag.ContinueOnError)
	file := fs.String("file", "", "JSONL job file")
	model := fs.String("model", configuredDefaultModel(), "default model")
	size := fs.String("size", "1024x1024", "default image size")
	quality := fs.String("quality", configuredDefaultQuality(), "default quality")
	format := fs.String("output-format", "png", "default output format")
	background := fs.String("background", "auto", "default background")
	moderation := fs.String("moderation", "auto", "default moderation")
	outDir := fs.String("out-dir", ".", "output directory")
	force := fs.Bool("force", false, "replace existing files")
	dryRun := fs.Bool("dry-run", false, "validate without network requests")
	timeout := fs.Duration("timeout", 5*time.Minute, "per-request timeout")
	attempts := fs.Int("max-attempts", 1, "request attempts, 1..3")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *file == "" {
		return errors.New("--file is required")
	}
	jobs, err := readJobs(*file)
	if err != nil {
		return err
	}
	client, err := newClient(*timeout, *attempts, *dryRun)
	if err != nil {
		return err
	}
	results := make([]map[string]any, 0, len(jobs))
	failures := 0
	for i, job := range jobs {
		opts := &generateOptions{
			Prompt: coalesce(job.Prompt, ""), Model: coalesce(job.Model, *model), Size: coalesce(job.Size, *size),
			Quality: coalesce(job.Quality, *quality), N: job.N, Background: coalesce(job.Background, *background),
			OutputFormat: coalesce(job.OutputFormat, *format), OutputCompression: -1, Moderation: coalesce(job.Moderation, *moderation),
			Out: job.Out, OutDir: *outDir, Force: *force, DryRun: *dryRun,
		}
		if opts.N == 0 {
			opts.N = 1
		}
		if job.OutputCompression != nil {
			opts.OutputCompression = *job.OutputCompression
		}
		if err := validateGenerate(opts); err != nil {
			failures++
			results = append(results, map[string]any{"line": i + 1, "ok": false, "error": sanitize(err.Error())})
			continue
		}
		if *dryRun {
			results = append(results, map[string]any{"line": i + 1, "ok": true, "dry_run": true, "model": opts.Model, "size": opts.Size, "quality": opts.Quality, "n": opts.N})
			continue
		}
		resp, err := generate(client, opts)
		if err != nil {
			failures++
			results = append(results, map[string]any{"line": i + 1, "ok": false, "error": sanitize(err.Error())})
			continue
		}
		files, err := saveImages(client, resp.Data, opts.Out, opts.OutDir, opts.OutputFormat, opts.Force)
		if err != nil {
			failures++
			results = append(results, map[string]any{"line": i + 1, "ok": false, "error": sanitize(err.Error())})
			continue
		}
		results = append(results, map[string]any{"line": i + 1, "ok": true, "model": opts.Model, "files": files, "usage": resp.Usage})
	}
	if err := writeJSON(map[string]any{"ok": failures == 0, "command": "generate-batch", "dry_run": *dryRun, "jobs": len(jobs), "failures": failures, "results": results}); err != nil {
		return err
	}
	if failures > 0 {
		return fmt.Errorf("%d batch job(s) failed", failures)
	}
	return nil
}

func readJobs(path string) ([]batchJob, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var jobs []batchJob
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		var job batchJob
		if err := json.Unmarshal([]byte(text), &job); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		if strings.TrimSpace(job.Prompt) == "" {
			return nil, fmt.Errorf("line %d: prompt is required", line)
		}
		jobs = append(jobs, job)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(jobs) == 0 {
		return nil, errors.New("batch file contains no jobs")
	}
	return jobs, nil
}

func (c *apiClient) requestWithRetry(build func() (*http.Request, error)) ([]byte, error) {
	var lastErr error
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		req, err := build()
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		body, status, requestID, err := c.do(req)
		if err == nil && status >= 200 && status < 300 {
			return body, nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = apiStatusError(status, body, requestID)
			if !retryableStatus(status) {
				return nil, lastErr
			}
		}
		if attempt < c.maxAttempts {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}
	return nil, lastErr
}

func (c *apiClient) do(req *http.Request) ([]byte, int, string, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if err != nil {
		return nil, resp.StatusCode, resp.Header.Get("x-request-id"), err
	}
	if len(body) > maxBodyBytes {
		return nil, resp.StatusCode, resp.Header.Get("x-request-id"), errors.New("API response exceeded 96 MiB")
	}
	return body, resp.StatusCode, resp.Header.Get("x-request-id"), nil
}

func retryableStatus(status int) bool {
	return status == 429 || status == 502 || status == 503 || status == 504 || status == 524
}

func apiStatusError(status int, body []byte, requestID string) error {
	message := strings.TrimSpace(string(body))
	var parsed struct {
		Error   any    `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &parsed) == nil {
		if parsed.Message != "" {
			message = parsed.Message
		} else if parsed.Error != nil {
			if b, err := json.Marshal(parsed.Error); err == nil {
				message = string(b)
			}
		}
	}
	message = sanitize(message)
	if len(message) > 1000 {
		message = message[:1000] + "…"
	}
	if requestID != "" {
		return fmt.Errorf("API status %d: %s (request_id=%s)", status, message, sanitize(requestID))
	}
	return fmt.Errorf("API status %d: %s", status, message)
}

func parseImageResponse(body []byte) (*imageResponse, error) {
	var resp imageResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("invalid image response: %w", err)
	}
	if len(resp.Data) == 0 {
		return nil, errors.New("image response contains no data")
	}
	return &resp, nil
}

func saveImages(client *apiClient, data []imageDatum, out, outDir, format string, force bool) ([]string, error) {
	ext := map[string]string{"png": ".png", "jpeg": ".jpg", "webp": ".webp"}[format]
	if ext == "" {
		ext = ".png"
	}
	if outDir == "" {
		outDir = "."
	}
	var paths []string
	for i, item := range data {
		path := out
		if path == "" {
			path = filepath.Join(outDir, fmt.Sprintf("salcara-%s%s", time.Now().Format("20060102-150405.000"), ext))
		} else if !filepath.IsAbs(path) {
			path = filepath.Join(outDir, path)
		}
		if len(data) > 1 {
			path = numberedPath(path, i+1)
		}
		var content []byte
		var err error
		if item.B64JSON != "" {
			content, err = base64.StdEncoding.DecodeString(item.B64JSON)
		} else if item.URL != "" {
			content, err = downloadImage(client.httpClient, item.URL)
		} else {
			err = errors.New("image item has neither b64_json nor url")
		}
		if err != nil {
			return nil, fmt.Errorf("image %d: %w", i+1, err)
		}
		if err := writeFile(path, content, force); err != nil {
			return nil, err
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		paths = append(paths, abs)
	}
	return paths, nil
}

func downloadImage(client *http.Client, rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return nil, errors.New("API returned an invalid image URL")
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("image download status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxImageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxImageBytes {
		return nil, errors.New("downloaded image exceeded 64 MiB")
	}
	return body, nil
}

func writeFile(path string, content []byte, force bool) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	flags := os.O_WRONLY | os.O_CREATE
	if force {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}
	f, err := os.OpenFile(path, flags, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("output exists: %s (use --force to replace)", path)
		}
		return err
	}
	if _, err = f.Write(content); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func numberedPath(path string, index int) string {
	ext := filepath.Ext(path)
	return strings.TrimSuffix(path, ext) + fmt.Sprintf("-%d", index) + ext
}

func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

func coalesce(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func sanitize(value string) string {
	return secretPattern.ReplaceAllString(value, "[REDACTED]")
}

func writeJSON(value any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}
