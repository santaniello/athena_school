package study

import (
	"bytes"
	"fmt"
	"text/template"
)

const explanationTemplate = `You are Athena, a technical learning assistant.
Explain the topic "{{.Topic}}" clearly and concisely for a backend developer.
Cover the key concepts, when to use it, and common trade-offs.
Keep the explanation under 300 words.`

const questionTemplate = `Now ask the user one specific question to test their understanding of "{{.Topic}}".
Ask only the question — no explanation.`

const evaluationTemplate = `The user answered the following question about "{{.Topic}}":

Question: {{.Question}}
Answer: {{.Answer}}

Evaluate the answer using this format:

✅ Strengths:
- <point>

⚠️ Improvements:
- <point>

⭐ Score: <n>/10`

type explanationData struct{ Topic string }
type questionData struct{ Topic string }
type evaluationData struct {
	Topic    string
	Question string
	Answer   string
}

// topicLabel returns "topic" or "topic › subtopic" depending on whether subtopic is set.
func topicLabel(topic, subtopic string) string {
	if subtopic != "" {
		return fmt.Sprintf("%s › %s", topic, subtopic)
	}
	return topic
}

func renderTemplate(tmplStr string, data any) (string, error) {
	tmpl, err := template.New("").Parse(tmplStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// BuildExplanationPrompt renders the explanation system prompt for the given topic and subtopic.
func BuildExplanationPrompt(topic, subtopic string) (string, error) {
	return renderTemplate(explanationTemplate, explanationData{Topic: topicLabel(topic, subtopic)})
}

// BuildQuestionPrompt renders the question prompt for the given topic label.
func BuildQuestionPrompt(topic string) (string, error) {
	return renderTemplate(questionTemplate, questionData{Topic: topic})
}

// BuildEvaluationPrompt renders the evaluation prompt with the given topic, question, and user answer.
func BuildEvaluationPrompt(topic, question, answer string) (string, error) {
	return renderTemplate(evaluationTemplate, evaluationData{
		Topic:    topic,
		Question: question,
		Answer:   answer,
	})
}
