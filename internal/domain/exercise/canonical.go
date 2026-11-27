package exercise

import (
	"errors"
	"maps"
	"slices"
)

// ErrCanonicalProblemNotFound is returned when no canonical problem has the id.
var ErrCanonicalProblemNotFound = errors.New("canonical problem not found")

// LocalizedName is a text shipped in both assistant languages.
type LocalizedName struct {
	EN string
	PT string
}

// For returns the text in the given assistant language; anything but "pt"
// reads as English.
func (n LocalizedName) For(language string) string {
	if language == "pt" {
		return n.PT
	}
	return n.EN
}

// CanonicalProblem is a classic system-design interview problem shipped with
// the app. It is trusted data: the model never chooses it. Core and Scale are
// English and are only fed to the model. See
// specs/phases/phase-07-improve-arch-lab/03-canonical-interviews.md.
type CanonicalProblem struct {
	ID      string // stable slug
	Name    LocalizedName
	Summary LocalizedName // one line shown in the catalog row
	Level   Difficulty    // how demanding the classic problem is, shown as its tag in the catalog
	Core    []string      // what the system must do, 2 to 4 lines
	Scale   map[Difficulty]string
}

// CanonicalProblems returns the problems in their fixed catalog order. The
// result is a copy: callers may change it freely.
func CanonicalProblems() []CanonicalProblem {
	problems := canonicalProblems()
	for i := range problems {
		problems[i].Core = slices.Clone(problems[i].Core)
		problems[i].Scale = maps.Clone(problems[i].Scale)
	}
	return problems
}

// FindCanonicalProblem returns the problem with the given id, or
// ErrCanonicalProblemNotFound.
func FindCanonicalProblem(id string) (CanonicalProblem, error) {
	for _, problem := range CanonicalProblems() {
		if problem.ID == id {
			return problem, nil
		}
	}
	return CanonicalProblem{}, ErrCanonicalProblemNotFound
}

func canonicalProblems() []CanonicalProblem {
	return []CanonicalProblem{
		{
			ID:      "url-shortener",
			Level:   DifficultyBeginner,
			Name:    LocalizedName{EN: "URL shortener", PT: "Encurtador de URL"},
			Summary: LocalizedName{EN: "Turn long URLs into short links and redirect them.", PT: "Encurte URLs longas e redirecione os links curtos."},
			Core: []string{
				"Create a short URL from a long one",
				"Optionally choose a custom alias and an expiration",
				"Redirect a short URL to the original",
			},
			Scale: scale("10 million stored URLs, 1 million daily users", "1 billion stored URLs, 100 million daily users", "1 billion stored URLs, 100 million daily users, served from several regions"),
		},
		{
			ID:      "rate-limiter",
			Level:   DifficultyIntermediate,
			Name:    LocalizedName{EN: "Rate limiter", PT: "Limitador de taxa"},
			Summary: LocalizedName{EN: "Limit how many requests each client can make.", PT: "Limite quantas requisições cada cliente pode fazer."},
			Core: []string{
				"Limit the requests of each client by identity, such as user, API key or IP address",
				"Reject the requests over the limit with a clear answer",
				"Let each endpoint have its own limits",
			},
			Scale: scale("1 thousand requests per second, 10 thousand clients", "100 thousand requests per second, 10 million clients", "1 million requests per second, 100 million clients, served from several regions"),
		},
		{
			ID:      "news-feed",
			Level:   DifficultyAdvanced,
			Name:    LocalizedName{EN: "News feed", PT: "Feed de notícias"},
			Summary: LocalizedName{EN: "Show each user the latest posts from the people they follow.", PT: "Mostre a cada usuário os posts mais recentes de quem ele segue."},
			Core: []string{
				"Let users publish posts",
				"Let users follow other users",
				"Show each user a feed of the latest posts from the people they follow",
			},
			Scale: scale("1 million users, 100 thousand daily posts", "100 million users, 10 million daily posts", "500 million users, 100 million daily posts, served from several regions"),
		},
		{
			ID:      "chat-application",
			Level:   DifficultyAdvanced,
			Name:    LocalizedName{EN: "Chat application", PT: "Aplicativo de chat"},
			Summary: LocalizedName{EN: "Send messages between users in real time, one to one and in groups.", PT: "Envie mensagens entre usuários em tempo real, em conversas privadas e em grupo."},
			Core: []string{
				"Send and receive messages in real time, one to one and in groups",
				"Keep the message history of each conversation",
				"Show whether a user is online and whether a message was delivered",
			},
			Scale: scale("100 thousand users, 1 million daily messages", "50 million users, 1 billion daily messages", "500 million users, 20 billion daily messages, served from several regions"),
		},
		{
			ID:      "notification-service",
			Level:   DifficultyIntermediate,
			Name:    LocalizedName{EN: "Notification service", PT: "Serviço de notificações"},
			Summary: LocalizedName{EN: "Deliver push, email and SMS notifications reliably.", PT: "Entregue notificações por push, e-mail e SMS com confiabilidade."},
			Core: []string{
				"Send notifications by push, email and SMS",
				"Let users choose which notifications they receive",
				"Retry a failed delivery and never send the same notification twice",
			},
			Scale: scale("1 million notifications per day", "100 million notifications per day", "1 billion notifications per day, served from several regions"),
		},
		{
			ID:      "ticket-booking",
			Level:   DifficultyIntermediate,
			Name:    LocalizedName{EN: "Ticket booking", PT: "Reserva de ingressos"},
			Summary: LocalizedName{EN: "Sell tickets for events without selling the same seat twice.", PT: "Venda ingressos para eventos sem vender o mesmo assento duas vezes."},
			Core: []string{
				"Search events and see the seats available",
				"Hold a seat while the user pays",
				"Never sell the same seat to two users",
			},
			Scale: scale("100 events, 1 thousand users at the same time", "10 thousand events, 100 thousand users at the same time on a popular event", "100 thousand events, 1 million users at the same time on a popular event, served from several regions"),
		},
	}
}

func scale(beginner, intermediate, advanced string) map[Difficulty]string {
	return map[Difficulty]string{
		DifficultyBeginner:     beginner,
		DifficultyIntermediate: intermediate,
		DifficultyAdvanced:     advanced,
	}
}
