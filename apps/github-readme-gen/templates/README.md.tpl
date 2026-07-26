### Hi there 👋

I'm toozej: a DevOps & Infrastructure engineer, audiophile, mountain fiend, photographer, car nerd, ~loud typist~ [mech keyboard enthusiast](https://github.com/toozej/keebs), FOSS-ist, skier, PNW-er. Once in IT, always in IT.

#### 👨💻 Repositories I created recently
{{range .RecentCreatedRepos}}
- [{{.Name}}]({{.URL}}) - {{.Description}}
{{- end}}

#### ⛏️ What I've been working on
{{range .RecentContributions}}
- [{{.Repo.Name}}]({{.Repo.URL}}) - {{.Repo.Description}} ({{humanize .OccurredAt}})
{{- end}}

#### ⭐ Recently starred repositories
{{range .RecentStarredRepos}}
- [{{.Name}}]({{.URL}}) - {{.Description}}
{{- end}}
