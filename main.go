package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	files       []string
	cursor      int
	selected    map[int]struct{}
	command     string
	quitting    bool
	currentDir  string
	folderCache map[string][]string
	page        int
	pageSize    int
	history     []string
	historyIdx  int
}

var (
	primaryColor   = lipgloss.Color("#E0E0E0")
	selectedColor  = lipgloss.Color("#C0C0C0")
	directoryColor = lipgloss.Color("#D0D0D0")
	fileColor      = lipgloss.Color("#F0F0F0")
	textColor      = lipgloss.Color("#FFFFFF")
)

func main() {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	folderCache := make(map[string][]string)
	files, err := getFiles(dir, folderCache)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	m := model{
		files:       files,
		selected:    make(map[int]struct{}),
		currentDir:  dir,
		folderCache: folderCache,
		page:        0,
		pageSize:    10,
		history:     []string{dir},
		historyIdx:  0,
	}
	p := tea.NewProgram(m)
	if err := p.Start(); err != nil {
		fmt.Println("Error:", err)
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", ":q":
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			if m.cursor < m.page*m.pageSize {
				m.page--
			}

		case "down", "j":
			if m.cursor < len(m.files)-1 {
				m.cursor++
			}
			if m.cursor >= (m.page+1)*m.pageSize {
				m.page++
			}

		case "left":
			if m.historyIdx > 0 {
				m.historyIdx--
				m.currentDir = m.history[m.historyIdx]
				files, _ := getFiles(m.currentDir, m.folderCache)
				m.files = files
				m.cursor = 0
				m.page = 0
			}

		case "right":
			if m.historyIdx < len(m.history)-1 {
				m.historyIdx++
				m.currentDir = m.history[m.historyIdx]
				files, _ := getFiles(m.currentDir, m.folderCache)
				m.files = files
				m.cursor = 0
				m.page = 0
			}

		case " ":
			_, ok := m.selected[m.cursor]
			if ok {
				delete(m.selected, m.cursor)
			} else {
				m.selected[m.cursor] = struct{}{}
			}

		case "enter":
			item := m.files[m.cursor]
			if isDirectory(item) {
				if err := os.Chdir(item); err == nil {
					m.history = append(m.history[:m.historyIdx+1], item)
					m.historyIdx++
					m.currentDir, _ = os.Getwd()
					files, _ := getFiles(m.currentDir, m.folderCache)
					m.files = files
					m.cursor = 0
					m.page = 0
				}
			} else if isReadable(item) {
				cmd := exec.Command("nvim", item)
				cmd.Stdin = os.Stdin
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				_ = cmd.Run()
			}

		case "backspace":
			if len(m.command) > 0 {
				m.command = m.command[:len(m.command)-1]
			}

		case ":":
			if len(m.command) > 0 {
				switch {
				case strings.HasPrefix(m.command, ":cd "):
					newDir := strings.TrimPrefix(m.command, ":cd ")
					if err := os.Chdir(newDir); err == nil {
						m.history = append(m.history[:m.historyIdx+1], newDir)
						m.historyIdx++
						m.currentDir, _ = os.Getwd()
						files, _ := getFiles(m.currentDir, m.folderCache)
						m.files = files
						m.cursor = 0
						m.page = 0
					}
					m.command = ""

				case strings.HasPrefix(m.command, ":w"):
					m.command = ""
				}
			}

		case ";":
			if len(m.command) > 0 {
				m.command = ""
			}

		case "pgup":
			if m.page > 0 {
				m.page--
			}

		case "pgdn":
			if (m.page+1)*m.pageSize < len(m.files) {
				m.page++
			}

		default:
			m.command += msg.String()
		}
	}

	return m, nil
}

func (m model) View() string {
	if m.quitting {
		return "Saliendo...\n"
	}

	var s strings.Builder
	headerStyle := lipgloss.NewStyle().Foreground(primaryColor).Bold(true)
	cursorStyle := lipgloss.NewStyle().Foreground(primaryColor)
	selectedStyle := lipgloss.NewStyle().Foreground(selectedColor)
	directoryStyle := lipgloss.NewStyle().Foreground(directoryColor)
	fileStyle := lipgloss.NewStyle().Foreground(fileColor)
	textStyle := lipgloss.NewStyle().Foreground(textColor)

	s.WriteString(headerStyle.Render("Dir: " + filepath.Base(m.currentDir) + " | Cmd: " + m.command + " | Pg: " + fmt.Sprintf("%d", m.page+1) + "\n"))

	start := m.page * m.pageSize
	end := start + m.pageSize
	if end > len(m.files) {
		end = len(m.files)
	}

	for i := start; i < end; i++ {
		item := m.files[i]
		cursor := " "
		if m.cursor == i {
			cursor = cursorStyle.Render("▶")
		}

		selected := " "
		if _, ok := m.selected[i]; ok {
			selected = selectedStyle.Render("✔")
		}

		icon := getIcon(item)
		if isDirectory(item) {
			item = directoryStyle.Render(filepath.Base(item))
		} else {
			item = fileStyle.Render(filepath.Base(item))
		}

		line := fmt.Sprintf("%s [%s] %s %s", cursor, selected, icon, textStyle.Render(item))
		s.WriteString(line + "\n")
	}

	return s.String()
}

func getFiles(dir string, folderCache map[string][]string) ([]string, error) {
	var files []string
	entries, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if isDirectory(entry) {
			if _, ok := folderCache[entry]; !ok {
				subFiles, _ := getFiles(entry, folderCache)
				folderCache[entry] = subFiles
			}
			files = append(files, entry)
		} else {
			files = append(files, entry)
		}
	}

	return files, nil
}

func isDirectory(file string) bool {
	info, err := os.Stat(file)
	return err == nil && info.IsDir()
}

func isReadable(file string) bool {
	extensions := []string{".txt", ".md", ".go", ".py", ".js", ".json", ".html", ".css", ".java", ".cpp", ".h", ".sh", ".rb", ".c", ".jsx", ".tsx", ".astro"}
	for _, ext := range extensions {
		if strings.HasSuffix(file, ext) {
			return true
		}
	}
	return false
}

func getIcon(file string) string {
	if isDirectory(file) {
		return colorize("📁", "#D0D0D0")
	}
	if strings.HasSuffix(file, ".go") {
		return colorize("", "#E0E0E0")
	} else if strings.HasSuffix(file, ".json") {
		return colorize("", "#FFFF00")
	} else if strings.HasSuffix(file, ".html") {
		return colorize("", "#FFA500")
	} else if strings.HasSuffix(file, ".md") {
		return colorize("", "#00FF00")
	} else if strings.HasSuffix(file, ".js") {
		return colorize("", "#FF0000")
	} else if strings.HasSuffix(file, ".css") {
		return colorize("", "#FF00FF")
	} else if strings.HasSuffix(file, ".py") {
		return colorize("", "#00FFFF")
	} else if strings.HasSuffix(file, ".java") {
		return colorize("", "#000080")
	} else if strings.HasSuffix(file, ".cpp") || strings.HasSuffix(file, ".h") {
		return colorize("", "#FFD700")
	} else if strings.HasSuffix(file, ".rb") {
		return colorize("", "#C8102E")
	} else if strings.HasSuffix(file, ".c") {
		return colorize("", "#4B0082")
	} else if strings.HasSuffix(file, ".sh") {
		return colorize("", "#FF4500")
	} else if strings.HasSuffix(file, ".jsx") {
		return colorize("", "#FF8C00")
	} else if strings.HasSuffix(file, ".tsx") {
		return colorize("", "#00BFFF")
	} else if strings.HasSuffix(file, ".astro") {
		return colorize("", "#8A2BE2")
	}
	return colorize("", "#A9A9A9")
}

func colorize(icon, color string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(icon)
}
