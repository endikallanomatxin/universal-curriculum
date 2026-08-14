package server

import (
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/models"
	"universal-curriculum/internal/server/guidance"
	"universal-curriculum/internal/server/releaseinfo"
	"universal-curriculum/internal/server/views"
	"universal-curriculum/internal/services"
)

func (server *Server) about(writer http.ResponseWriter, request *http.Request) {
	server.renderUserPage(writer, request, "about.html", "about", false)
}

func (server *Server) aboutCase(writer http.ResponseWriter, request *http.Request) {
	server.renderUserPage(writer, request, "case.html", "about", false)
}

func (server *Server) aboutProposal(writer http.ResponseWriter, request *http.Request) {
	server.renderUserPage(writer, request, "proposal.html", "about", false)
}

type aboutContentPage struct {
	Slug     string
	Anchor   string
	Title    string
	Date     string
	DateText string
	Summary  string
	Content  string
	Rendered template.HTML
}

type aboutContentSectionData struct {
	userPageData
	SectionSlug         string
	SectionTitle        string
	SectionIntroduction string
	Pages               []aboutContentPage
	Page                *aboutContentPage
	Continuous          bool
	Title               string
}

func (server *Server) releases(writer http.ResponseWriter, request *http.Request) {
	server.aboutReleaseSection(writer, request, "releases", "Releases", "Published changes, newest first.", releasePages(releaseinfo.Releases()))
}

func (server *Server) roadmap(writer http.ResponseWriter, request *http.Request) {
	server.aboutReleaseSection(writer, request, "roadmap", "Roadmap", "Planned direction, subject to change.", releasePages(releaseinfo.Roadmap()))
}

func releasePages(documents []releaseinfo.Document) []aboutContentPage {
	pages := make([]aboutContentPage, 0, len(documents))
	for _, document := range documents {
		content := document.Content
		if _, remainder, found := strings.Cut(content, "\n"); found && strings.HasPrefix(content, "# ") {
			content = strings.TrimSpace(remainder)
		}
		dateText := ""
		if document.Date != "" {
			date, err := time.Parse(time.DateOnly, document.Date)
			if err != nil {
				panic("invalid generated release date " + document.Date)
			}
			dateText = date.Format("2 January 2006")
		}
		pages = append(pages, aboutContentPage{
			Slug: document.Version, Anchor: "release-" + strings.ReplaceAll(document.Version, ".", "-"),
			Title: "v" + document.Version, Date: document.Date, DateText: dateText,
			Summary: document.Summary, Content: content,
		})
	}
	return pages
}

func (server *Server) aboutReleaseSection(writer http.ResponseWriter, request *http.Request, slug, title, introduction string, pages []aboutContentPage) {
	data, err := server.loadUserPageData(request, "about", false)
	if err != nil {
		http.Error(writer, "Load user", http.StatusInternalServerError)
		return
	}
	for index := range pages {
		pages[index].Rendered = views.RenderUnitContent(demoteMarkdownHeadings(pages[index].Content))
	}
	server.render(writer, "about-content.html", aboutContentSectionData{
		userPageData: data, SectionSlug: slug, SectionTitle: title,
		SectionIntroduction: introduction, Pages: pages, Continuous: true,
		Title: title + " · Universal Curriculum",
	})
}

func demoteMarkdownHeadings(content string) string {
	lines := strings.Split(content, "\n")
	inFence := false
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if !inFence && strings.HasPrefix(line, "#") {
			lines[index] = "#" + line
		}
	}
	return strings.Join(lines, "\n")
}

func (server *Server) aboutContentSection(writer http.ResponseWriter, request *http.Request, slug, title, introduction string, pages []aboutContentPage) {
	data, err := server.loadUserPageData(request, "about", false)
	if err != nil {
		http.Error(writer, "Load user", http.StatusInternalServerError)
		return
	}
	view := aboutContentSectionData{userPageData: data, SectionSlug: slug, SectionTitle: title, SectionIntroduction: introduction, Pages: pages, Title: title + " · Universal Curriculum"}
	if pageSlug := request.PathValue("slug"); pageSlug != "" {
		for index := range view.Pages {
			if view.Pages[index].Slug == pageSlug {
				view.Pages[index].Rendered = views.RenderUnitContent(view.Pages[index].Content)
				view.Page = &view.Pages[index]
				view.Title = view.Pages[index].Title + " · " + title + " · Universal Curriculum"
				server.render(writer, "about-content.html", view)
				return
			}
		}
		http.NotFound(writer, request)
		return
	}
	server.render(writer, "about-content.html", view)
}

func (server *Server) documentation(writer http.ResponseWriter, request *http.Request) {
	pages := guidance.Pages()
	contentPages := make([]aboutContentPage, 0, len(pages))
	for _, page := range pages {
		contentPages = append(contentPages, aboutContentPage{Slug: page.Slug, Title: page.Title, Summary: page.Summary, Content: page.Content})
	}
	server.aboutContentSection(writer, request, "documentation", "Documentation", "How the curriculum and its workflows work.", contentPages)
}

func (server *Server) license(writer http.ResponseWriter, request *http.Request) {
	server.renderUserPage(writer, request, "license.html", "about", false)
}

type homeLearningUnitRecommendation struct {
	models.Unit
	URL string
}

type homeLearningPathRecommendation struct {
	ID           int64
	Name         string
	URL          string
	PendingCount int
	NextUnits    []homeLearningUnitRecommendation
}

func (server *Server) homeLearningRecommendations(userID int64) ([]homeLearningPathRecommendation, error) {
	paths, err := db.ListLearningPaths(server.Database, userID)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, nil
	}
	graph, err := db.GetCurriculumGraph(server.Database)
	if err != nil {
		return nil, err
	}
	completedUnitIDs, err := db.CompletedUnitIDs(server.Database, userID)
	if err != nil {
		return nil, err
	}
	return newHomeLearningRecommendations(paths, graph, completedUnitIDs), nil
}

func newHomeLearningRecommendations(
	paths []models.LearningPath,
	graph *models.CurriculumGraph,
	completedUnitIDs map[int64]bool,
) []homeLearningPathRecommendation {
	recommendations := make([]homeLearningPathRecommendation, 0, len(paths))
	for _, path := range paths {
		targetIDs := make([]int64, 0, len(path.Units))
		for _, unit := range path.Units {
			targetIDs = append(targetIDs, unit.ID)
		}
		nextUnits, pendingCount := services.AvailableLearningPathUnits(graph, targetIDs, completedUnitIDs)
		if pendingCount == 0 || len(nextUnits) == 0 {
			continue
		}
		pathID := strconv.FormatInt(path.ID, 10)
		recommendation := homeLearningPathRecommendation{
			ID:           path.ID,
			Name:         path.Name,
			URL:          "/learn?path=" + pathID,
			PendingCount: pendingCount,
			NextUnits:    make([]homeLearningUnitRecommendation, 0, len(nextUnits)),
		}
		for _, unit := range nextUnits {
			unitID := strconv.FormatInt(unit.ID, 10)
			recommendation.NextUnits = append(recommendation.NextUnits, homeLearningUnitRecommendation{
				Unit: unit,
				URL:  "/learn?path=" + pathID + "&unit=" + unitID + "&content=" + unitID,
			})
		}
		recommendations = append(recommendations, recommendation)
	}
	return recommendations
}
