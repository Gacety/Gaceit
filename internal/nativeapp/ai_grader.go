package nativeapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

const aiConfigKey = "408-ai-config-v1"

// AIConfig configures an OpenAI-compatible chat completions endpoint used to
// grade subjective answers. Empty fields fall back to environment variables
// or standard defaults at call time.
type AIConfig struct {
	Endpoint string `json:"endpoint"`
	APIKey   string `json:"apiKey"`
	Model    string `json:"model"`
}

func DefaultAIConfig() AIConfig {
	return AIConfig{
		Endpoint: "https://api.openai.com/v1",
		Model:    "gpt-4o-mini",
	}
}

func (c AIConfig) resolved() AIConfig {
	out := c
	if strings.TrimSpace(out.Endpoint) == "" {
		out.Endpoint = firstNonEmptyEnv("STUDY_AI_ENDPOINT", "OPENAI_BASE_URL")
	}
	if strings.TrimSpace(out.Endpoint) == "" {
		out.Endpoint = "https://api.openai.com/v1"
	}
	out.Endpoint = strings.TrimRight(strings.TrimSpace(out.Endpoint), "/")
	if strings.TrimSpace(out.APIKey) == "" {
		out.APIKey = firstNonEmptyEnv("STUDY_AI_API_KEY", "OPENAI_API_KEY")
	}
	out.APIKey = strings.TrimSpace(out.APIKey)
	if strings.TrimSpace(out.Model) == "" {
		out.Model = firstNonEmptyEnv("STUDY_AI_MODEL", "OPENAI_MODEL")
	}
	if strings.TrimSpace(out.Model) == "" {
		out.Model = "gpt-4o-mini"
	}
	out.Model = strings.TrimSpace(out.Model)
	return out
}

func firstNonEmptyEnv(names ...string) string {
	for _, name := range names {
		if v := strings.TrimSpace(os.Getenv(name)); v != "" {
			return v
		}
	}
	return ""
}

func isLocalEndpoint(endpoint string) bool {
	lower := strings.ToLower(strings.TrimSpace(endpoint))
	return strings.HasPrefix(lower, "http://localhost") ||
		strings.HasPrefix(lower, "http://127.0.0.1") ||
		strings.HasPrefix(lower, "http://[::1]")
}

func (c AIConfig) Configured() bool {
	eff := c.resolved()
	if eff.Endpoint == "" || eff.Model == "" {
		return false
	}
	return eff.APIKey != "" || isLocalEndpoint(eff.Endpoint)
}

func (c AIConfig) hasCustomValues() bool {
	return strings.TrimSpace(c.Endpoint) != "" || strings.TrimSpace(c.APIKey) != "" || strings.TrimSpace(c.Model) != ""
}

type AIGradeResult struct {
	Score         float64  `json:"score"`
	MaxScore      float64  `json:"maxScore"`
	Outcome       string   `json:"outcome"`
	Feedback      string   `json:"feedback"`
	MatchedPoints []string `json:"matchedPoints,omitempty"`
	MissedPoints  []string `json:"missedPoints,omitempty"`
}

func buildChatCompletionsURL(endpoint string) string {
	base := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if strings.HasSuffix(base, "/chat/completions") {
		return base
	}
	return base + "/chat/completions"
}

var aiHTTPClient = &http.Client{Timeout: 45 * time.Second}

func GradeSubjectiveWithAI(ctx context.Context, cfg AIConfig, q Question, userAnswer string) (*AIGradeResult, error) {
	if strings.TrimSpace(userAnswer) == "" {
		return nil, fmt.Errorf("请先填写作答内容")
	}
	eff := cfg.resolved()
	if !eff.Configured() {
		return nil, fmt.Errorf("请先在「AI 设置」中配置 API Key 与模型")
	}
	systemPrompt := `你是一名严谨的计算机与考研专业课阅卷老师。请根据题干、参考答案、评分细则（如有）和解析，对考生提交的主观题答案进行客观批改。
请只返回一个合法的 JSON 对象，不要包含额外说明文字，字段如下：
{
  "score": 数字（当前得分）,
  "maxScore": 数字（满分，若评分细则未注明则默认为 10）,
  "outcome": 字符串，只能是 "right"、"uncertain" 或 "wrong" 之一：
    - "right": 核心思路与关键采分点基本完整正确（一般得分 >= 85%）
    - "uncertain": 大方向正确或部分采分点命中，但存在遗漏或表达不够严谨
    - "wrong": 核心概念错误、方向跑偏或未答出关键点
  "matchedPoints": ["已命中的采分点1", "..."],
  "missedPoints": ["遗漏或错误的点1", "..."],
  "feedback": "简明扼要的中文批改评语与改进建议"
}`
	var prompt strings.Builder
	fmt.Fprintf(&prompt, "【学科/章节】%s / %s\n", q.Subject, q.Chapter)
	if q.Title != "" {
		fmt.Fprintf(&prompt, "【题目标题】%s\n", q.Title)
	}
	fmt.Fprintf(&prompt, "【题干】\n%s\n\n", q.Text)
	if strings.TrimSpace(q.Answer) != "" {
		fmt.Fprintf(&prompt, "【参考答案】\n%s\n\n", q.Answer)
	}
	if strings.TrimSpace(q.Rubric) != "" {
		fmt.Fprintf(&prompt, "【评分细则】\n%s\n\n", q.Rubric)
	}
	if strings.TrimSpace(q.Explanation) != "" {
		fmt.Fprintf(&prompt, "【解析参考】\n%s\n\n", q.Explanation)
	}
	fmt.Fprintf(&prompt, "【考生作答】\n%s\n", userAnswer)

	payload, err := json.Marshal(map[string]any{
		"model":       eff.Model,
		"temperature": 0.2,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": prompt.String()},
		},
	})
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, buildChatCompletionsURL(eff.Endpoint), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if eff.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+eff.APIKey)
	}
	resp, err := aiHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 AI 接口失败：%w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("读取 AI 响应失败：%w", err)
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("解析 AI 响应失败（HTTP %d）：%w", resp.StatusCode, err)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return nil, fmt.Errorf("AI 接口返回错误：%s", parsed.Error.Message)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("AI 接口状态异常（HTTP %d）", resp.StatusCode)
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return nil, fmt.Errorf("AI 未返回有效的批改结果")
	}
	return parseAIGradeJSON(parsed.Choices[0].Message.Content)
}

func parseAIGradeJSON(raw string) (*AIGradeResult, error) {
	text := strings.TrimSpace(raw)
	if start := strings.Index(text, "{"); start >= 0 {
		if end := strings.LastIndex(text, "}"); end > start {
			text = text[start : end+1]
		}
	}
	var out AIGradeResult
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return nil, fmt.Errorf("AI 返回格式不是有效 JSON：%w", err)
	}
	if out.MaxScore <= 0 {
		out.MaxScore = 10
	}
	if out.Score < 0 {
		out.Score = 0
	}
	if out.Score > out.MaxScore {
		out.Score = out.MaxScore
	}
	out.Outcome = normalizeSubjectiveOutcome(out.Outcome)
	if out.Outcome == "unknown" {
		out.Outcome = "wrong"
	}
	out.Feedback = strings.TrimSpace(out.Feedback)
	return &out, nil
}

type aiGradeResponse struct {
	qID    string
	answer string
	grade  *AIGradeResult
	err    error
}

func (a *App) startAIGrade() {
	if a.aiGrading {
		return
	}
	q := *a.M.Current()
	answer := a.subjectiveText
	if strings.TrimSpace(answer) == "" {
		a.message = "请填写作答内容后再提交"
		return
	}
	if !a.M.AIConfig.Configured() {
		a.aiShowConfig = true
		a.message = "请先在下方填写 API 地址、API Key 与模型名称"
		return
	}
	a.message = ""
	a.aiGrading = true
	ch := make(chan aiGradeResponse, 1)
	a.aiGradeCh = ch
	cfg := a.M.AIConfig
	window := a.Window
	go func() {
		grade, err := GradeSubjectiveWithAI(context.Background(), cfg, q, answer)
		ch <- aiGradeResponse{qID: q.ID, answer: answer, grade: grade, err: err}
		if window != nil {
			window.Invalidate()
		}
	}()
}

func (a *App) collectAIGrade(c *ui.Context) {
	if a.aiGradeCh == nil {
		return
	}
	select {
	case res := <-a.aiGradeCh:
		a.aiGradeCh = nil
		a.aiGrading = false
		if res.err != nil {
			a.message = res.err.Error()
			return
		}
		if a.M.Current().ID != res.qID {
			return
		}
		a.flushInk()
		if a.inkDirty {
			return
		}
		a.collectSave(true)
		correct, err := a.M.SubmitSubjective(res.answer, res.grade.Outcome, res.grade, time.Now())
		if err != nil {
			a.message = err.Error()
			return
		}
		a.answered = true
		a.correct = correct
		a.unknown = false
		a.lastGrade = res.grade
		a.message = ""
	default:
		if a.aiGrading {
			c.After(100 * time.Millisecond)
		}
	}
}

func (a *App) submitSubjectiveOutcome(outcome string) {
	a.flushInk()
	if a.inkDirty {
		return
	}
	a.collectSave(true)
	correct, err := a.M.SubmitSubjective(a.subjectiveText, outcome, nil, time.Now())
	if err != nil {
		a.message = err.Error()
		return
	}
	a.answered = true
	a.correct = correct
	a.unknown = outcome == "unknown"
	a.lastGrade = nil
	a.message = ""
}

func (a *App) overrideSubjective(outcome string) {
	a.collectSave(true)
	if err := a.M.OverrideLastSubjectiveOutcome(outcome, time.Now()); err != nil {
		a.message = err.Error()
		return
	}
	outcome = normalizeSubjectiveOutcome(outcome)
	a.correct = outcome == "right"
	a.unknown = outcome == "unknown"
	if a.lastGrade != nil {
		a.lastGrade.Outcome = outcome
	}
	a.message = ""
}

func (a *App) subjectiveQuestionSection(c *ui.Context, q *Question, r *Record) {
	t := c.Theme()
	ui.Column(c.Key("subjective-" + q.ID)).FillWidth().Gap(a.qs(12)).Children(func() {
		if !a.answered {
			ui.Text(c, "你的解答").FontSize(a.qs(14)).FontWeight(700).TextColor(t.TextMuted)
			area := ui.TextArea(c.Key("subjective-answer-"+q.ID), &a.subjectiveText).
				Label("主观题作答输入").
				Placeholder("在此输入你的解答思路、计算步骤或代码…").
				FillWidth().
				MinHeight(a.qs(140)).
				Padding(a.qs(14)).
				FontSize(a.qs(16)).
				Radius(a.qs(12)).
				Background(c.Theme().Background).
				Border(a.qs(1), t.Border)
			_ = area
			ui.Row(c).Gap(a.qs(8)).Wrap().AlignItems(ui.Center).Children(func() {
				aiLabel := "AI 批改"
				if a.aiGrading {
					aiLabel = "AI 批改中…"
				}
				if a.exactButton(c, aiLabel).
					Height(a.qs(38)).
					Padding(0, a.qs(16)).
					Radius(a.qs(10)).
					Background(t.Accent).
					TextColor(t.AccentText).
					FontSize(a.qs(14)).
					FontWeight(700).
					Disabled(a.aiGrading).
					Clicked() {
					a.startAIGrade()
				}
				for _, selfItem := range []struct{ outcome, label string }{
					{"right", "自评：掌握"},
					{"uncertain", "自评：模糊"},
					{"unknown", "不会，先看解析"},
				} {
					if a.exactButton(c, selfItem.label).
						Height(a.qs(38)).
						Padding(0, a.qs(12)).
						Radius(a.qs(10)).
						Border(a.qs(1), t.Border).
						Background(t.Surface).
						TextColor(t.Text).
						FontSize(a.qs(13)).
						Disabled(a.aiGrading).
						Clicked() {
						a.submitSubjectiveOutcome(selfItem.outcome)
					}
				}
				ui.Spacer(c)
				cfgBtn := a.exactButton(c, "AI 设置").
					Height(a.qs(38)).
					Padding(0, a.qs(12)).
					Radius(a.qs(10)).
					Border(a.qs(1), t.Border).
					FontSize(a.qs(13))
				if a.aiShowConfig {
					cfgBtn.Background(a.exactColor("#2A2545", "#e4e9df")).TextColor(a.exactColor("#E4DAFF", "#465b48"))
				}
				if cfgBtn.Clicked() {
					a.aiShowConfig = !a.aiShowConfig
				}
			})
			if a.aiShowConfig {
				a.aiConfigPanel(c)
			}
			return
		}
		if strings.TrimSpace(a.subjectiveText) != "" {
			ui.Column(c).FillWidth().Padding(a.qs(14)).Gap(a.qs(6)).Radius(a.qs(12)).Background(t.Background).Border(a.qs(1), t.Border).Children(func() {
				ui.Text(c, "你的作答").FontSize(a.qs(13)).FontWeight(700).TextColor(t.TextMuted)
				ui.Text(c, a.subjectiveText).FontSize(a.qs(15)).LineHeight(1.6).Selectable()
			})
		}
	})
}

func (a *App) aiConfigPanel(c *ui.Context) {
	t := c.Theme()
	ui.Column(c.Key("ai-config-panel")).FillWidth().Padding(a.qs(16)).Gap(a.qs(10)).Radius(a.qs(12)).Background(t.Background).Border(a.qs(1), t.Border).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "AI 批改接口设置（OpenAI 兼容格式）").FontSize(a.qs(14)).FontWeight(700).Grow(1)
			ui.Text(c, "留空项将尝试读取环境变量").FontSize(a.qs(12)).TextColor(t.TextMuted)
		})
		if ui.TextInput(c.Key("ai-endpoint"), &a.M.AIConfig.Endpoint).Label("API 地址").Placeholder("https://api.openai.com/v1").FillWidth().Changed() {
			a.saveSoon()
		}
		if ui.TextInput(c.Key("ai-key"), &a.M.AIConfig.APIKey).Label("API Key").Placeholder("sk-...").FillWidth().Changed() {
			a.saveSoon()
		}
		if ui.TextInput(c.Key("ai-model"), &a.M.AIConfig.Model).Label("模型名称").Placeholder("gpt-4o-mini / deepseek-chat / qwen-plus").FillWidth().Changed() {
			a.saveSoon()
		}
		ui.Row(c).Gap(a.qs(8)).AlignItems(ui.Center).Children(func() {
			if a.exactButton(c, "保存设置").Height(a.qs(34)).Padding(0, a.qs(14)).Radius(a.qs(8)).Background(t.Accent).TextColor(t.AccentText).FontWeight(700).Clicked() {
				a.save()
				a.aiShowConfig = false
				a.message = ""
			}
			if a.exactButton(c, "收起").Height(a.qs(34)).Padding(0, a.qs(12)).Radius(a.qs(8)).Border(a.qs(1), t.Border).Clicked() {
				a.aiShowConfig = false
			}
		})
	})
}

func (a *App) subjectiveAnsweredSection(c *ui.Context, q *Question, r *Record) {
	t := c.Theme()
	outcome := "wrong"
	if len(r.Attempts) > 0 {
		if s, ok := r.Attempts[len(r.Attempts)-1]["outcome"].(string); ok {
			outcome = s
		}
	}
	title := "需加强练习（安排 10 分钟后短期重学）"
	color := t.Danger
	switch outcome {
	case "right":
		title = "判定掌握 · 按复习阶梯推进"
		color = t.Success
	case "uncertain":
		title = "部分掌握 / 模糊 · 安排 24 小时后复习"
		color = t.Warning
	case "unknown":
		title = "本次标记不会 · 参考答案如下"
		color = t.Danger
	}
	ui.Text(c, title).FontSize(a.qs(17)).FontWeight(700).TextColor(color)
	if a.lastGrade != nil {
		ui.Column(c).FillWidth().Padding(a.qs(14)).Gap(a.qs(8)).Radius(a.qs(12)).Background(t.Background).Border(a.qs(1), t.Border).Children(func() {
			ui.Row(c).Gap(a.qs(10)).AlignItems(ui.Center).Children(func() {
				ui.Textf(c, "AI 评分：%.1f / %.1f", a.lastGrade.Score, a.lastGrade.MaxScore).FontSize(a.qs(15)).FontWeight(700).TextColor(color)
			})
			if a.lastGrade.Feedback != "" {
				ui.Text(c, a.lastGrade.Feedback).FontSize(a.qs(14)).LineHeight(1.6).Selectable()
			}
			if len(a.lastGrade.MatchedPoints) > 0 {
				ui.Text(c, "已命中采分点："+strings.Join(a.lastGrade.MatchedPoints, "；")).FontSize(a.qs(13)).TextColor(t.Success).LineHeight(1.5).Selectable()
			}
			if len(a.lastGrade.MissedPoints) > 0 {
				ui.Text(c, "遗漏/扣分点："+strings.Join(a.lastGrade.MissedPoints, "；")).FontSize(a.qs(13)).TextColor(t.Danger).LineHeight(1.5).Selectable()
			}
		})
	}
	ui.Row(c).Gap(a.qs(8)).Wrap().AlignItems(ui.Center).Children(func() {
		ui.Text(c, "人工改判：").FontSize(a.qs(12)).TextColor(t.TextMuted)
		for _, item := range []struct{ outcome, label string }{
			{"right", "改判：掌握"},
			{"uncertain", "改判：模糊"},
			{"wrong", "改判：答错"},
		} {
			b := a.exactButton(c, item.label).Height(a.qs(30)).Padding(0, a.qs(10)).Radius(a.qs(8)).FontSize(a.qs(12)).Border(a.qs(1), t.Border).Disabled(a.M.Before == nil)
			if outcome == item.outcome {
				b.Background(a.exactColor("#2A2545", "#e4e9df")).TextColor(a.exactColor("#E4DAFF", "#465b48")).Border(a.qs(1), t.Accent)
			}
			if b.Clicked() {
				a.overrideSubjective(item.outcome)
			}
		}
	})
	if strings.TrimSpace(q.Answer) != "" {
		ui.Column(c).FillWidth().Gap(a.qs(4)).Children(func() {
			ui.Text(c, "参考答案").FontSize(a.qs(13)).FontWeight(700).TextColor(t.TextMuted)
			a.Rich.View(c, q.Answer, a.qs(16)).Label("主观题参考答案")
		})
	}
	if strings.TrimSpace(q.Rubric) != "" {
		ui.Column(c).FillWidth().Gap(a.qs(4)).Children(func() {
			ui.Text(c, "评分细则").FontSize(a.qs(13)).FontWeight(700).TextColor(t.TextMuted)
			a.Rich.View(c, q.Rubric, a.qs(14)).Label("评分细则")
		})
	}
}
