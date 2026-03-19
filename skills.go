package oneagent

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	skillSourceBuiltIn = "built-in"

	skillPriorityEmbedded = iota
	skillPriorityProjectClient
	skillPriorityProjectShared
	skillPriorityUserClient
	skillPriorityUserShared

	skillMaxDepth = 6
	skillMaxDirs  = 2000
)

// Skill is a parsed skill definition from either the embedded defaults or a
// discovered custom SKILL.md file.
type Skill struct {
	Name        string
	Description string
	Body        string
	Directory   string
	Location    string
	Source      string
	Priority    int
	Embedded    bool
}

type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type skillRoot struct {
	Path     string
	Source   string
	Priority int
}

var errStopSkillScan = errors.New("stop skill scan")

var skippedSkillDirs = map[string]struct{}{
	".git":         {},
	".hg":          {},
	".svn":         {},
	".next":        {},
	"build":        {},
	"coverage":     {},
	"dist":         {},
	"node_modules": {},
	"out":          {},
	"target":       {},
	"tmp":          {},
	"vendor":       {},
}

//go:embed defaults/skills/*/SKILL.md
var embeddedSkillsFS embed.FS

// LoadEmbeddedSkills returns the built-in skills that ship in the binary.
func LoadEmbeddedSkills() ([]Skill, error) {
	paths, err := fs.Glob(embeddedSkillsFS, "defaults/skills/*/SKILL.md")
	if err != nil {
		return nil, err
	}

	sort.Strings(paths)
	skills := make([]Skill, 0, len(paths))
	for _, skillPath := range paths {
		skill, err := loadEmbeddedSkill(skillPath)
		if err != nil {
			return nil, fmt.Errorf("load embedded skill %q: %w", skillPath, err)
		}
		skills = append(skills, skill)
	}
	return skills, nil
}

// LoadSkills returns the merged embedded and discovered skill catalog for a
// working directory. When extraDirs is non-empty, those directories replace
// the default user-level scan paths (project-level scanning is unaffected).
func LoadSkills(cwd string, extraDirs ...string) (map[string]Skill, error) {
	embedded, err := LoadEmbeddedSkills()
	if err != nil {
		return nil, err
	}
	return MergeSkills(embedded, DiscoverSkills(cwd, extraDirs...)), nil
}

// DiscoverSkills returns custom skills discovered on disk. When extraDirs is
// non-empty, those directories replace the default user-level scan paths.
func DiscoverSkills(cwd string, extraDirs ...string) []Skill {
	roots := skillRoots(cwd, extraDirs)
	skills := make([]Skill, 0)
	for _, root := range roots {
		skills = append(skills, discoverSkillRoot(root)...)
	}
	return skills
}

// MergeSkills merges embedded and discovered skills with first-match-wins
// precedence. Embedded skills are always inserted first.
func MergeSkills(embedded, discovered []Skill) map[string]Skill {
	merged := make(map[string]Skill, len(embedded)+len(discovered))
	addSkills(merged, sortSkillsByPriority(embedded))
	addSkills(merged, sortSkillsByPriority(discovered))
	return merged
}

// BuildSkillCatalog renders the compact skill catalog used for session-start
// disclosure.
func BuildSkillCatalog(skills map[string]Skill) string {
	if len(skills) == 0 {
		return ""
	}

	lines := []string{
		"The following skills provide specialized instructions for specific tasks.",
		"When a task matches a skill description, run 'oa skills show <name>' to load its full instructions.",
		"",
		"Available skills:",
	}
	for _, name := range sortedSkillNames(skills) {
		lines = append(lines, fmt.Sprintf("- %s: %s", name, skills[name].Description))
	}
	return strings.Join(lines, "\n")
}

// InjectSkillCatalog prepends the merged skill catalog ahead of an existing
// system prompt. If no skills are available, the original prompt is returned.
// When skillDirs is non-empty, those directories replace the default user-level
// scan paths.
func InjectSkillCatalog(systemPrompt, cwd string, skillDirs []string) (string, error) {
	skills, err := LoadSkills(cwd, skillDirs...)
	if err != nil {
		return "", err
	}

	catalog := BuildSkillCatalog(skills)
	if catalog == "" {
		return systemPrompt, nil
	}
	if systemPrompt == "" {
		return catalog, nil
	}
	return catalog + "\n\n" + systemPrompt, nil
}

func addSkills(merged map[string]Skill, skills []Skill) {
	for _, skill := range skills {
		if existing, ok := merged[skill.Name]; ok {
			log.Printf("skill %q from %s shadowed by %s", skill.Name, skill.Source, existing.Source)
			continue
		}
		merged[skill.Name] = skill
	}
}

// ListSkillResources returns bundled files in a skill directory other than
// SKILL.md. Embedded skills are listed from the embedded filesystem.
func ListSkillResources(skill Skill) ([]string, error) {
	if skill.Embedded {
		return listSkillResourcesFS(embeddedSkillsFS, skill.Directory)
	}
	return listSkillResourcesDir(skill.Directory)
}

func skillRoots(cwd string, extraDirs []string) []skillRoot {
	roots := make([]skillRoot, 0, 4)
	if resolved := resolveSkillCWD(cwd); resolved != "" {
		roots = append(roots,
			skillRoot{
				Path:     filepath.Join(resolved, ".oneagent", "skills"),
				Source:   ".oneagent/skills/",
				Priority: skillPriorityProjectClient,
			},
			skillRoot{
				Path:     filepath.Join(resolved, ".agents", "skills"),
				Source:   ".agents/skills/",
				Priority: skillPriorityProjectShared,
			},
		)
	}

	if len(extraDirs) > 0 {
		for _, dir := range extraDirs {
			roots = append(roots, skillRoot{
				Path:     dir,
				Source:   dir + "/",
				Priority: skillPriorityUserClient,
			})
		}
		return roots
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return roots
	}

	return append(roots,
		skillRoot{
			Path:     filepath.Join(home, ".config", "oneagent", "skills"),
			Source:   "~/.config/oneagent/skills/",
			Priority: skillPriorityUserClient,
		},
		skillRoot{
			Path:     filepath.Join(home, ".agents", "skills"),
			Source:   "~/.agents/skills/",
			Priority: skillPriorityUserShared,
		},
	)
}

func resolveSkillCWD(cwd string) string {
	if cwd == "" {
		resolved, err := os.Getwd()
		if err != nil {
			return ""
		}
		cwd = resolved
	}

	abs, err := filepath.Abs(cwd)
	if err != nil {
		return cwd
	}
	return abs
}

func discoverSkillRoot(root skillRoot) []Skill {
	if !skillRootExists(root.Path) {
		return nil
	}

	scan := skillScan{
		root:   root,
		seen:   make(map[string]string),
		skills: make([]Skill, 0),
	}
	err := filepath.WalkDir(root.Path, scan.walk)
	if err != nil && !errors.Is(err, errStopSkillScan) {
		log.Printf("scan skill root %s: %v", root.Path, err)
	}
	return scan.skills
}

type skillScan struct {
	root     skillRoot
	seen     map[string]string
	skills   []Skill
	dirsSeen int
}

func skillRootExists(path string) bool {
	info, err := os.Stat(path)
	switch {
	case err == nil:
		return info.IsDir()
	case os.IsNotExist(err):
		return false
	default:
		log.Printf("skip skill root %s: %v", path, err)
		return false
	}
}

func (s *skillScan) walk(current string, entry fs.DirEntry, walkErr error) error {
	if handled, err := handleSkillWalkErr(current, entry, walkErr); handled {
		return err
	}
	if entry.IsDir() {
		return visitSkillDir(s.root.Path, current, entry.Name(), &s.dirsSeen)
	}
	if entry.Name() != "SKILL.md" {
		return nil
	}
	return s.recordSkill(current)
}

func handleSkillWalkErr(current string, entry fs.DirEntry, walkErr error) (bool, error) {
	if walkErr == nil {
		return false, nil
	}
	log.Printf("skip skill path %s: %v", current, walkErr)
	if entry != nil && entry.IsDir() {
		return true, fs.SkipDir
	}
	return true, nil
}

func (s *skillScan) recordSkill(skillPath string) error {
	skill, ok := loadCustomSkill(skillPath, s.root)
	if !ok {
		return nil
	}
	if existing, ok := s.seen[skill.Name]; ok {
		log.Printf("skill %q at %s shadowed by %s", skill.Name, skill.Location, existing)
		return nil
	}
	s.seen[skill.Name] = skill.Location
	s.skills = append(s.skills, skill)
	return nil
}

func visitSkillDir(root, current, name string, dirsSeen *int) error {
	*dirsSeen = *dirsSeen + 1
	if *dirsSeen > skillMaxDirs {
		log.Printf("skill scan limit reached under %s after %d directories", root, skillMaxDirs)
		return errStopSkillScan
	}
	if current != root {
		if _, ok := skippedSkillDirs[name]; ok {
			return fs.SkipDir
		}
	}
	if skillDepth(root, current) > skillMaxDepth {
		return fs.SkipDir
	}
	return nil
}

func skillDepth(root, current string) int {
	rel, err := filepath.Rel(root, current)
	if err != nil || rel == "." {
		return 0
	}
	return len(strings.Split(filepath.ToSlash(rel), "/"))
}

func loadCustomSkill(skillPath string, root skillRoot) (Skill, bool) {
	data, err := os.ReadFile(skillPath)
	if err != nil {
		log.Printf("read skill %s: %v", skillPath, err)
		return Skill{}, false
	}

	absPath, err := filepath.Abs(skillPath)
	if err != nil {
		absPath = skillPath
	}
	dir := filepath.Dir(absPath)
	fallbackName := filepath.Base(dir)
	meta, body, err := parseSkillDocument(string(data), fallbackName)
	if err != nil {
		log.Printf("skip skill %s: %v", absPath, err)
		return Skill{}, false
	}

	warnOnSkillMetadata(meta, fallbackName, absPath)
	return Skill{
		Name:        meta.Name,
		Description: meta.Description,
		Body:        body,
		Directory:   dir,
		Location:    absPath,
		Source:      root.Source,
		Priority:    root.Priority,
	}, true
}

func loadEmbeddedSkill(skillPath string) (Skill, error) {
	data, err := fs.ReadFile(embeddedSkillsFS, skillPath)
	if err != nil {
		return Skill{}, err
	}

	name := path.Base(path.Dir(skillPath))
	meta, body, err := parseSkillDocument(string(data), name)
	if err != nil {
		return Skill{}, err
	}

	return Skill{
		Name:        meta.Name,
		Description: meta.Description,
		Body:        body,
		Directory:   path.Dir(skillPath),
		Location:    skillPath,
		Source:      skillSourceBuiltIn,
		Priority:    skillPriorityEmbedded,
		Embedded:    true,
	}, nil
}

func parseSkillDocument(content, fallbackName string) (skillFrontmatter, string, error) {
	frontmatter, body, err := splitFrontmatter(content)
	if err != nil {
		return skillFrontmatter{}, "", err
	}

	meta, err := unmarshalSkillFrontmatter(frontmatter)
	if err != nil {
		return skillFrontmatter{}, "", fmt.Errorf("parse frontmatter: %w", err)
	}

	if meta.Name == "" {
		meta.Name = fallbackName
	}
	if meta.Description == "" {
		return skillFrontmatter{}, "", fmt.Errorf("missing description")
	}

	return meta, trimSkillBody(body), nil
}

func unmarshalSkillFrontmatter(frontmatter string) (skillFrontmatter, error) {
	var meta skillFrontmatter
	err := yaml.Unmarshal([]byte(frontmatter), &meta)
	if err == nil {
		return meta, nil
	}

	relaxed := relaxSkillFrontmatter(frontmatter)
	if relaxed == frontmatter {
		return skillFrontmatter{}, err
	}
	if retryErr := yaml.Unmarshal([]byte(relaxed), &meta); retryErr != nil {
		return skillFrontmatter{}, err
	}
	return meta, nil
}

func relaxSkillFrontmatter(frontmatter string) string {
	lines := strings.Split(frontmatter, "\n")
	changed := false

	for i, line := range lines {
		key, value, ok := splitSkillFrontmatterLine(line)
		if !ok || !strings.Contains(value, ":") || isQuotedOrBlockValue(value) {
			continue
		}
		lines[i] = fmt.Sprintf("%s%s: %q", leadingWhitespace(line), key, value)
		changed = true
	}
	if !changed {
		return frontmatter
	}
	return strings.Join(lines, "\n")
}

func splitSkillFrontmatterLine(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "- ") {
		return "", "", false
	}

	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", "", false
	}

	key := strings.TrimSpace(line[:idx])
	value := strings.TrimSpace(line[idx+1:])
	if key == "" || value == "" {
		return "", "", false
	}
	return key, value, true
}

func isQuotedOrBlockValue(value string) bool {
	switch value[0] {
	case '"', '\'', '|', '>':
		return true
	default:
		return false
	}
}

func leadingWhitespace(line string) string {
	return line[:len(line)-len(strings.TrimLeft(line, " \t"))]
}

func splitFrontmatter(content string) (string, string, error) {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return "", "", fmt.Errorf("missing YAML frontmatter")
	}

	parts := strings.SplitN(normalized[4:], "\n---\n", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("missing frontmatter terminator")
	}
	return parts[0], parts[1], nil
}

func trimSkillBody(body string) string {
	return strings.Trim(strings.TrimLeft(body, "\n"), "\n")
}

func warnOnSkillMetadata(meta skillFrontmatter, fallbackName, location string) {
	if meta.Name != fallbackName {
		log.Printf("skill %q does not match directory %q at %s", meta.Name, fallbackName, location)
	}
	if len(meta.Name) > 64 {
		log.Printf("skill %q exceeds 64 characters at %s", meta.Name, location)
	}
}

func sortSkillsByPriority(skills []Skill) []Skill {
	sorted := append([]Skill(nil), skills...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Priority < sorted[j].Priority
	})
	return sorted
}

func sortedSkillNames(skills map[string]Skill) []string {
	names := make([]string, 0, len(skills))
	for name := range skills {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func listSkillResourcesDir(dir string) ([]string, error) {
	entries := make([]string, 0)
	err := filepath.WalkDir(dir, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Name() == "SKILL.md" {
			return nil
		}
		rel, err := filepath.Rel(dir, current)
		if err != nil {
			return err
		}
		entries = append(entries, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	sort.Strings(entries)
	return entries, nil
}

func listSkillResourcesFS(fsys fs.FS, dir string) ([]string, error) {
	entries := make([]string, 0)
	err := fs.WalkDir(fsys, dir, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if path.Base(current) == "SKILL.md" {
			return nil
		}
		rel := strings.TrimPrefix(current, dir+"/")
		entries = append(entries, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(entries)
	return entries, nil
}
