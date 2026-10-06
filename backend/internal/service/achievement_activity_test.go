package service

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAchievementQuizRequiresEightAnswersAndDoesNotLeakSolutions(t *testing.T) {
	topics := AchievementTopics()
	if len(topics) != 12 {
		t.Fatalf("topics=%d", len(topics))
	}
	for _, topic := range topics {
		q, e := AchievementQuiz(topic.Kind, topic.Key)
		if e != nil {
			t.Fatal(e)
		}
		if len(q.Questions) != 10 {
			t.Fatalf("topic %s questions %d", topic.Key, len(q.Questions))
		}
		raw, _ := json.Marshal(q)
		if strings.Contains(string(raw), "correct") {
			t.Fatal("answer key leaked")
		}
		answers := make([]int, 10)
		for i, b := range achievementBanks[topic.Kind+":"+topic.Key] {
			answers[i] = b.Correct
		}
		result, e := GradeAchievementQuiz(topic.Kind, topic.Key, answers)
		if e != nil || result.Score != 10 || !result.Passed {
			t.Fatal("correct answers rejected")
		}
		answers[0] = (answers[0] + 1) % 3
		answers[1] = (answers[1] + 1) % 3
		answers[2] = (answers[2] + 1) % 3
		result, e = GradeAchievementQuiz(topic.Kind, topic.Key, answers)
		if e != nil || result.Passed {
			t.Fatal("seven passed")
		}
		if _, e = GradeAchievementQuiz(topic.Kind, topic.Key, answers[:9]); e == nil {
			t.Fatal("short submission accepted")
		}
	}
	if _, e := AchievementQuiz("theme", "unknown"); e == nil {
		t.Fatal("invalid topic accepted")
	}
}

func TestAchievementQuizzesHaveNoUniversalPositionalShortcut(t *testing.T) {
	universal := []int{0, 1, 2, 0, 1, 2, 0, 1, 2, 0}
	passed := 0
	for _, topic := range AchievementTopics() {
		r, e := GradeAchievementQuiz(topic.Kind, topic.Key, universal)
		if e != nil {
			t.Fatal(e)
		}
		if r.Passed {
			passed++
		}
	}
	if passed == len(AchievementTopics()) {
		t.Fatal("same positional vector passes every topic")
	}
}
