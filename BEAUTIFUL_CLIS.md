# Making Beautiful, Lightweight CLIs

## Philosophy

Beautiful CLIs are:
- **Fast**: Instant feedback, no lag
- **Clear**: Information hierarchy is obvious
- **Consistent**: Predictable patterns throughout
- **Accessible**: Works in all environments (TTY, non-TTY, screen readers)
- **Progressive**: Beautiful when styled, functional when not

Lightweight means:
- **No heavy dependencies**: Minimal external libraries
- **Fast startup**: No initialization overhead
- **Small binary size**: Essential features only
- **Low memory**: Efficient rendering

## Design Principles

### 1. Progressive Enhancement

Start with plain text, enhance with styling:

```go
// Works beautifully without any styling
fmt.Fprintf(ctx.App.Out, "User: %s\n", username)

// Enhanced with styling (optional)
if ctx.App.Styles.ChildName != nil {
    styled := ctx.App.Styles.ChildName.Render(username)
    fmt.Fprintf(ctx.App.Out, "User: %s\n", styled)
}
```

**Rule**: Every component must work perfectly in plain text mode.

### 2. Information Hierarchy

Use visual hierarchy to guide the eye:

```
┌─────────────────────────────────────┐
│  App Title (Bold, Accent Color)     │  ← Most important
├─────────────────────────────────────┤
│  Section Heading (Bold)             │  ← Secondary
│    • Item (Normal)                  │  ← Tertiary
│      Description (Muted)           │  ← Least important
└─────────────────────────────────────┘
```

**Implementation**:
- **Titles**: Bold, accent color, larger (if possible)
- **Headings**: Bold, primary color
- **Body**: Normal weight, default color
- **Muted**: Dimmed, secondary color

### 3. Consistent Spacing

Use consistent spacing patterns:

```go
// Vertical rhythm
const (
    SpacingTight   = 0  // No spacing
    SpacingNormal  = 1  // Single line
    SpacingLoose   = 2  // Double line
)

// Horizontal rhythm
const (
    IndentLevel1 = 2  // 2 spaces
    IndentLevel2 = 4  // 4 spaces
    IndentLevel3 = 6  // 6 spaces
)
```

**Pattern**: Use 2-space indentation for nested content, single blank lines between sections, double blank lines between major sections.

### 4. Color Palette

Use a limited, semantic color palette:

```go
// Semantic colors (not literal colors)
type ColorSemantic int

const (
    ColorPrimary   ColorSemantic = iota // Main actions, titles
    ColorSecondary                      // Secondary info
    ColorSuccess                        // Success states
    ColorWarning                        // Warnings
    ColorError                          // Errors
    ColorMuted                          // Less important text
    ColorAccent                         // Highlights, emphasis
)

// Map to actual colors based on theme
func (c ColorSemantic) ToColor(theme Theme) lipgloss.Color {
    switch c {
    case ColorPrimary:
        return theme.Primary
    case ColorSuccess:
        return theme.Success
    // ...
    }
}
```

**Rule**: Use colors semantically, not decoratively. Colors should convey meaning.

### 5. Typography

Use typography to create hierarchy:

```go
// Typography scale
type TypographyScale struct {
    Title    TextStyle // Largest, boldest
    Heading  TextStyle // Large, bold
    Body     TextStyle // Normal
    Small    TextStyle // Smaller, muted
    Code     TextStyle // Monospace, distinct
}
```

**Patterns**:
- **Titles**: Bold, larger (if terminal supports)
- **Headings**: Bold
- **Body**: Normal weight
- **Code**: Monospace, distinct background
- **Labels**: Slightly emphasized

## Component Design Patterns

### 1. Cards and Containers

Use borders and padding to group related information:

```go
// Card component
func RenderCard(w io.Writer, opts CardOptions) {
    if isTerminal(w) {
        // Styled with borders
        border := lipgloss.NewStyle().
            Border(lipgloss.RoundedBorder()).
            BorderForeground(lipgloss.Color("63")).
            Padding(1, 2)
        fmt.Fprint(w, border.Render(content))
    } else {
        // ASCII fallback
        fmt.Fprintf(w, "+%s+\n", strings.Repeat("-", width-2))
        fmt.Fprintf(w, "| %s |\n", content)
        fmt.Fprintf(w, "+%s+\n", strings.Repeat("-", width-2))
    }
}
```

**Use cases**:
- Grouping related information
- Highlighting important content
- Creating visual separation

### 2. Lists and Tables

Use alignment and spacing for readability:

```go
// List with consistent spacing
func RenderList(w io.Writer, items []ListItem) {
    for _, item := range items {
        // Bullet + indent + label + description
        fmt.Fprintf(w, "  %s %-20s %s\n",
            bullet,
            item.Label,
            item.Description,
        )
    }
}

// Table with aligned columns
func RenderTable(w io.Writer, headers []string, rows [][]string) {
    // Calculate column widths
    widths := calculateWidths(headers, rows)
    
    // Render header
    renderRow(w, headers, widths, true) // bold
    
    // Render separator
    renderSeparator(w, widths)
    
    // Render rows
    for _, row := range rows {
        renderRow(w, row, widths, false)
    }
}
```

**Patterns**:
- Align columns for tables
- Consistent bullet styles for lists
- Indentation for hierarchy

### 3. Status Indicators

Use symbols and colors for quick recognition:

```go
// Status indicators
const (
    StatusSuccess = "✓"  // Green
    StatusError   = "✗"  // Red
    StatusWarning = "⚠"  // Yellow
    StatusInfo    = "ℹ"  // Blue
    StatusPending = "…"  // Gray
)

func RenderStatus(w io.Writer, status Status, label string) {
    symbol := status.Symbol()
    color := status.Color()
    
    if isTerminal(w) {
        styled := lipgloss.NewStyle().
            Foreground(color).
            Render(symbol)
        fmt.Fprintf(w, "%s %s\n", styled, label)
    } else {
        fmt.Fprintf(w, "%s %s\n", symbol, label)
    }
}
```

**Patterns**:
- Use Unicode symbols (with ASCII fallbacks)
- Color-code by status type
- Keep symbols simple and recognizable

### 4. Progress and Loading

Provide feedback for long operations:

```go
// Progress bar
func RenderProgress(w io.Writer, current, total int, label string) {
    percent := float64(current) / float64(total)
    barWidth := 40
    filled := int(percent * float64(barWidth))
    
    bar := strings.Repeat("█", filled) +
           strings.Repeat("░", barWidth-filled)
    
    if isTerminal(w) {
        styled := lipgloss.NewStyle().
            Foreground(lipgloss.Color("2")). // Green
            Render(bar)
        fmt.Fprintf(w, "%s [%s] %d%%\n", label, styled, int(percent*100))
    } else {
        fmt.Fprintf(w, "%s [%s] %d%%\n", label, bar, int(percent*100))
    }
}

// Spinner (animated in terminal, static in text)
func RenderSpinner(w io.Writer, message string) {
    frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
    
    if isTerminal(w) {
        // Animated (requires cursor control)
        // Use ANSI codes to update in place
    } else {
        // Static
        fmt.Fprintf(w, "[*] %s\n", message)
    }
}
```

**Patterns**:
- Show progress for operations > 1 second
- Use spinners for indeterminate progress
- Update in place (don't spam output)

## Default Themes

Provide sensible defaults that work well:

```go
// Default theme presets
var (
    ThemeDefault = Theme{
        Primary:   lipgloss.Color("63"),   // Purple
        Secondary: lipgloss.Color("147"),  // Light blue
        Success:   lipgloss.Color("2"),    // Green
        Warning:   lipgloss.Color("3"),    // Yellow
        Error:     lipgloss.Color("1"),    // Red
        Muted:     lipgloss.Color("8"),    // Gray
        Accent:    lipgloss.Color("205"),  // Pink
    }
    
    ThemeMinimal = Theme{
        // Monochrome, subtle
        Primary:   lipgloss.Color("7"),   // White
        Secondary: lipgloss.Color("8"),   // Gray
        Success:   lipgloss.Color("2"),    // Green
        Warning:   lipgloss.Color("3"),    // Yellow
        Error:     lipgloss.Color("1"),    // Red
        Muted:     lipgloss.Color("8"),    // Gray
        Accent:    lipgloss.Color("7"),    // White
    }
    
    ThemeColorful = Theme{
        // Vibrant, high contrast
        Primary:   lipgloss.Color("213"),  // Bright purple
        Secondary: lipgloss.Color("147"),  // Cyan
        Success:   lipgloss.Color("46"),   // Bright green
        Warning:   lipgloss.Color("226"),  // Bright yellow
        Error:     lipgloss.Color("196"),   // Bright red
        Muted:     lipgloss.Color("244"),   // Dark gray
        Accent:    lipgloss.Color("212"),  // Bright pink
    }
)
```

## Performance Considerations

### 1. Lazy Rendering

Don't render until needed:

```go
// Bad: Renders immediately
card := RenderCard(opts) // Allocates string

// Good: Renders on write
func RenderCard(w io.Writer, opts CardOptions) {
    // Only renders when writing
}
```

### 2. Reuse Styles

Cache style objects:

```go
var (
    titleStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("63"))
    // Reuse this, don't create new ones
)
```

### 3. Minimize Allocations

Use string builders for complex output:

```go
func RenderTable(w io.Writer, data TableData) {
    var buf strings.Builder
    buf.Grow(estimateSize(data)) // Pre-allocate
    
    // Build output
    renderHeader(&buf, data)
    renderRows(&buf, data)
    
    // Write once
    fmt.Fprint(w, buf.String())
}
```

### 4. Detect Capabilities Once

Cache terminal detection:

```go
type Renderer struct {
    isTerminal bool
    width      int
    // Cached on first use
}

func (r *Renderer) detectCapabilities(w io.Writer) {
    if r.isTerminal == nil {
        r.isTerminal = isTerminal(w)
        r.width = getTerminalWidth(w)
    }
}
```

## Best Practices

### 1. Output Formatting

```go
// ✅ Good: Clear, scannable
fmt.Fprintf(w, "  %s: %s\n", label, value)
fmt.Fprintf(w, "  %s: %s\n", label2, value2)

// ❌ Bad: Hard to scan
fmt.Fprintf(w, "%s:%s\n%s:%s\n", label, value, label2, value2)
```

### 2. Error Messages

```go
// ✅ Good: Clear, actionable
fmt.Fprintf(w, "Error: %s\n", err)
fmt.Fprintf(w, "  Hint: %s\n", hint)

// ❌ Bad: Cryptic
fmt.Fprintf(w, "ERR: %v\n", err)
```

### 3. Success Messages

```go
// ✅ Good: Positive, clear
fmt.Fprintf(w, "✓ Successfully created %s\n", resource)

// ❌ Bad: Too verbose
fmt.Fprintf(w, "The operation completed successfully. The resource %s was created.\n", resource)
```

### 4. Progress Updates

```go
// ✅ Good: Updates in place
updateProgress(w, current, total)

// ❌ Bad: Spams output
for i := 0; i < total; i++ {
    fmt.Fprintf(w, "Progress: %d/%d\n", i, total)
}
```

## Component Composition

Build complex UIs from simple components:

```go
// Compose components
func RenderDashboard(w io.Writer, data DashboardData) {
    // Header
    RenderCard(w, CardOptions{
        Title: "System Status",
        Content: renderStatusSection(data.Status),
    })
    
    // Separator
    RenderSeparator(w, SeparatorOptions{Length: 40})
    
    // Metrics
    RenderTable(w, TableOptions{
        Headers: []string{"Metric", "Value", "Status"},
        Rows: data.Metrics,
    })
    
    // Footer
    RenderKeyValue(w, []KeyValue{
        {Key: "Last Updated", Value: data.LastUpdated},
    })
}
```

## Accessibility

### 1. Screen Reader Support

```go
// Provide text alternatives
func RenderStatus(w io.Writer, status Status) {
    if isScreenReader(w) {
        // Text-only
        fmt.Fprintf(w, "Status: %s\n", status.Text())
    } else {
        // Visual with symbol
        fmt.Fprintf(w, "%s %s\n", status.Symbol(), status.Text())
    }
}
```

### 2. Color Independence

Never rely solely on color:

```go
// ✅ Good: Symbol + color
fmt.Fprintf(w, "%s %s\n", "✓", "Success") // Green

// ❌ Bad: Color only
fmt.Fprintf(w, "%s\n", "Success") // Just green text
```

### 3. High Contrast

Ensure sufficient contrast:

```go
// Use high-contrast colors
const (
    ColorText      = lipgloss.Color("7")  // White on dark
    ColorBackground = lipgloss.Color("0")  // Black
    // Contrast ratio: 12.6:1 (WCAG AAA)
)
```

## Authentication Flow Examples

### Beautiful Auth Code Display

```go
func renderAuthCode(w io.Writer, code string) {
    // Prominent, easy-to-copy format
    clix.RenderCard(w, clix.CardOptions{
        Title: "Authentication Code",
        Content: fmt.Sprintf(`
Copy this code before opening your browser:

  %s

This code expires in 10 minutes.
`, code),
        Border: true,
    })
    
    // Auto-copy to clipboard (TerminalPrompter only)
    if isTerminal(w) {
        copyToClipboard(code)
        fmt.Fprintln(w, "✓ Code copied to clipboard")
    }
}
```

### Beautiful Browser Auth Flow

```go
func runBrowserAuth(w io.Writer, authURL string) error {
    // Step 1: Show message
    fmt.Fprintf(w, "Opening browser for authentication...\n")
    
    // Step 2: Open browser
    openBrowser(authURL)
    
    // Step 3: Show waiting state with spinner
    spinner := clix.NewSpinner("Waiting for authentication...")
    spinner.Start()
    defer spinner.Stop()
    
    // Step 4: Start local server
    server := startLocalServer()
    defer server.Close()
    
    // Step 5: Wait for callback
    token := <-server.TokenChan
    
    // Step 6: Show success
    spinner.Stop()
    clix.RenderAlert(w, clix.AlertOptions{
        Type:    clix.AlertSuccess,
        Title:   "Success",
        Message: "Authentication successful!",
    })
    
    return nil
}
```

### Beautiful Auth Status

```go
func renderAuthStatus(w io.Writer, status AuthStatus) {
    if !status.Authenticated {
        clix.RenderEmptyState(w, clix.EmptyStateOptions{
            Icon:    "🔒",
            Title:   "Not authenticated",
            Message: "Run 'cli auth login' to authenticate",
        })
        return
    }
    
    // Show authenticated state
    clix.RenderCard(w, clix.CardOptions{
        Title: "Authentication Status",
        Content: renderStatusContent(status),
    })
    
    // Show service account details
    if status.ServiceAccount != "" {
        clix.RenderKeyValue(w, []clix.KeyValue{
            {Key: "Service Account", Value: status.ServiceAccount},
            {Key: "Expires", Value: formatExpiry(status.TokenExpires)},
            {Key: "Scopes", Value: strings.Join(status.Scopes, ", ")},
        })
    }
}
```

## Examples

### Beautiful Command Output

```go
func (cmd *ListCommand) Run(ctx *clix.Context) error {
    items := cmd.fetchItems()
    
    if len(items) == 0 {
        RenderEmptyState(ctx.App.Out, EmptyStateOptions{
            Icon:    "📭",
            Title:   "No items found",
            Message: "Try creating an item to get started.",
        })
        return nil
    }
    
    // Header
    RenderCard(ctx.App.Out, CardOptions{
        Title: fmt.Sprintf("Items (%d)", len(items)),
    })
    
    // List
    listItems := make([]ListItem, len(items))
    for i, item := range items {
        listItems[i] = ListItem{
            Label:       item.Name,
            Description: item.Description,
            Value:       item.ID,
        }
    }
    
    RenderList(ctx.App.Out, listItems, ListOptions{
        Style: ListVertical,
        Bullet: "•",
    })
    
    return nil
}
```

### Beautiful Error Handling

```go
func handleError(w io.Writer, err error) {
    RenderAlert(w, AlertOptions{
        Type:    AlertError,
        Title:   "Error",
        Message: err.Error(),
    })
    
    // Provide actionable hints
    if isValidationError(err) {
        fmt.Fprintf(w, "\n  Hint: Check your input and try again.\n")
    } else if isNetworkError(err) {
        fmt.Fprintf(w, "\n  Hint: Check your network connection.\n")
    }
}
```

## Summary

Beautiful, lightweight CLIs are:

1. **Progressive**: Work without styling, beautiful with styling
2. **Hierarchical**: Clear information structure
3. **Consistent**: Predictable patterns
4. **Fast**: Minimal overhead, efficient rendering
5. **Accessible**: Works everywhere, for everyone

The key is to start simple and enhance progressively, always maintaining functionality even without styling.

