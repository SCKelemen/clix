# clix Components Specification

## Overview

This document specifies the component system for clix, designed to provide rich, interactive UI components for CLI applications while maintaining simplicity and compatibility with both interactive (TerminalPrompter) and non-interactive (TextPrompter) modes.

## Design Principles

1. **Dual-Mode Support**: All components must work with both:
   - `TerminalPrompter` (TTY control, rich interactivity)
   - `TextPrompter` (line-based, fallback for non-TTY environments)

2. **Lightweight**: No event loop model (unlike Bubble Tea). Components are prompt-and-return, not long-running applications.

3. **Accessible**: Follow patterns from Radix UI and Slack Block Kit for accessibility and standard component semantics.

4. **Composable**: Components can be used standalone or composed in Surveys.

5. **Terminating**: Components prompt, return a value, and exit. They don't take over the terminal permanently.

6. **Styled with Lipgloss**: All components use `TextStyle` interface (compatible with `lipgloss.Style`) via `app.Styles` hooks.

7. **Validated with Extension**: All input components use `ext/validation` package for validation logic.

## Component Categories

Components are divided into two categories:

1. **Input Components**: Prompt the user for input, return a value via `Prompter.Prompt()`
2. **Display Components**: Render information to output via `io.Writer`, no user input

## Component Types

### Core Input Components

#### 1. Text Input
**Status**: ✅ Implemented
- Single-line text input
- Placeholder/default values
- Validation
- Autocomplete suggestions (TerminalPrompter only)

#### 2. Textarea
**Status**: ⚠️ Partially implemented (via prompt extension)
- Multi-line text input
- Line wrapping
- Scrollable for long content
- **Enhancement needed**: Better multi-line editing in TerminalPrompter

#### 3. Password Input
**Status**: ❌ Not implemented
- Masked input (shows `*` or `•`)
- Optional reveal toggle (TerminalPrompter only)
- **API**:
  ```go
  clix.PromptRequest{
      Label: "Password",
      Type: clix.PromptPassword,
      Password: clix.PasswordOptions{
          Mask: true,        // Mask input (default: true)
          MaskChar: "•",     // Character for masking (default: "*")
          RevealKey: "Ctrl+R", // Toggle reveal key (TerminalPrompter only)
      },
      Validate: validation.MinLength(8),
  }
  ```

#### 4. Number Input
**Status**: ⚠️ Partially implemented (via validation)
- Numeric input with type validation
- Min/max constraints
- Step increment (for TerminalPrompter: arrow keys)
- Integer or float mode
- **API**:
  ```go
  clix.PromptRequest{
      Label: "Age",
      Type: clix.PromptNumber,
      Number: clix.NumberOptions{
          Min: ptr(0.0),   // Optional minimum (nil = no min)
          Max: ptr(120.0), // Optional maximum (nil = no max)
          Step: 1.0,       // Step increment (default: 1.0)
          Int: true,       // Integer-only mode (default: false)
      },
      Validate: validation.Integer,
  }
  ```

#### 5. Combobox
**Status**: ❌ Not implemented
- Text input with autocomplete from options list
- Filterable dropdown (TerminalPrompter)
- Free-form text allowed (unlike Select)
- **API**:
  ```go
  clix.PromptRequest{
      Label: "Country",
      Type: clix.PromptCombobox,
      Options: []clix.SelectOption{
          {Label: "United States", Value: "US"},
          {Label: "Canada", Value: "CA"},
      },
      Combobox: clix.ComboboxOptions{
          AllowCustom: true,  // Allow values not in options (default: false)
          Filterable: true,   // Enable filtering (TerminalPrompter only, default: true)
          MinChars: 0,        // Minimum chars before filtering (default: 0)
      },
  }
  ```

### Selection Components

#### 6. Select (Single Choice)
**Status**: ✅ Implemented
- Single selection from options
- Arrow key navigation (TerminalPrompter)
- Search/filter (TerminalPrompter)

#### 7. MultiSelect
**Status**: ✅ Implemented
- Multiple selections from options
- Toggle with Space (TerminalPrompter)
- Visual checkmarks

#### 8. Radio Group
**Status**: ❌ Not implemented
- Single selection from options (similar to Select but different visual style)
- Horizontal or vertical layout
- **API**:
  ```go
  clix.PromptRequest{
      Label: "Choose option",
      Type: clix.PromptRadio,
      Options: []clix.SelectOption{...},
      Layout: clix.RadioVertical, // or RadioHorizontal
  }
  ```

#### 9. Checkbox Group
**Status**: ⚠️ Partially implemented (via MultiSelect)
- Multiple selections with explicit checkboxes
- Different from MultiSelect: always shows all options, no filtering
- **API**:
  ```go
  clix.PromptRequest{
      Label: "Select features",
      Type: clix.PromptCheckbox,
      Options: []clix.SelectOption{...},
  }
  ```

### Confirmation Components

#### 10. Confirm
**Status**: ✅ Implemented
- Yes/No prompt
- Custom button text

### Advanced Components

#### 11. File Picker
**Status**: ❌ Not implemented
- Navigate file system
- Filter by extension
- Directory navigation (TerminalPrompter)
- **API**:
  ```go
  clix.PromptRequest{
      Label: "Select file",
      Type: clix.PromptFile,
      File: clix.FileOptions{
          Filter: []string{".go", ".txt"}, // Optional file extensions
          Directory: false,                  // true for directory picker (default: false)
          StartDir: "~/.config",            // Starting directory (default: current dir)
          ShowHidden: false,                // Show hidden files (default: false)
      },
  }
  ```

#### 12. Date Picker
**Status**: ❌ Not implemented
- Date selection
- Calendar view (TerminalPrompter)
- Text input fallback (TextPrompter)
- **API**:
  ```go
  clix.PromptRequest{
      Label: "Birth date",
      Type: clix.PromptDate,
      Date: clix.DateOptions{
          Format: "2006-01-02", // Go date format (default: "2006-01-02")
          Min: time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC), // Optional
          Max: time.Now(),                                    // Optional
          WeekStart: time.Monday,                            // Week start day (default: Sunday)
      },
  }
  ```

#### 13. Time Picker
**Status**: ❌ Not implemented
- Time selection
- 12/24 hour format
- **API**:
  ```go
  clix.PromptRequest{
      Label: "Meeting time",
      Type: clix.PromptTime,
      Time: clix.TimeOptions{
          Format: "15:04",     // Go time format (default: "15:04" = 24h)
          Hour12: false,       // 12-hour format (default: false = 24h)
          ShowSeconds: false,  // Include seconds (default: false)
      },
  }
  ```

#### 14. Slider/Range
**Status**: ❌ Not implemented
- Numeric value selection via slider
- Visual feedback (TerminalPrompter)
- Arrow key adjustment
- **API**:
  ```go
  clix.PromptRequest{
      Label: "Volume",
      Type: clix.PromptSlider,
      Slider: clix.SliderOptions{
          Min: 0.0,      // Minimum value
          Max: 100.0,    // Maximum value
          Step: 1.0,     // Step increment (default: 1.0)
          Default: 50.0, // Default value
          ShowValue: true, // Show current value (default: true)
          Width: 40,      // Slider width in chars (TerminalPrompter only, default: 40)
      },
  }
  ```

### Display Components

Display components render information to output, not for user input. They write to an `io.Writer` (typically `ctx.App.Out` or `os.Stdout`).

#### 15. List Display
**Status**: ❌ Not implemented
- Render items as a list
- Multiple layout styles (vertical, horizontal, card)
- **API**:
  ```go
  items := []clix.ListItem{
      {Label: "Item 1", Value: "value1"},
      {Label: "Item 2", Value: "value2"},
  }
  
  // Vertical list (default)
  clix.RenderList(ctx.App.Out, items, clix.ListOptions{
      Style: clix.ListVertical,
      Bullet: "•", // or "-", "*", etc.
  })
  
  // Horizontal list
  clix.RenderList(ctx.App.Out, items, clix.ListOptions{
      Style: clix.ListHorizontal,
      Separator: ", ",
  })
  
  // Card list (TerminalPrompter: styled boxes, TextPrompter: indented)
  clix.RenderList(ctx.App.Out, items, clix.ListOptions{
      Style: clix.ListCard,
      Title: "Available Options",
  })
  ```

#### 16. Card Display
**Status**: ❌ Not implemented
- Render content in a card/box
- Bordered, styled container
- **API**:
  ```go
  clix.RenderCard(ctx.App.Out, clix.CardOptions{
      Title: "User Information",
      Content: "Name: John Doe\nEmail: john@example.com",
      Border: true,
      Padding: 1,
  })
  ```

#### 17. Table Display
**Status**: ❌ Not implemented
- Tabular data display
- Sortable columns (TerminalPrompter)
- Pagination
- **API**:
  ```go
  table := clix.NewTable(clix.TableOptions{
      Headers: []string{"Name", "Age", "City"},
      Rows: [][]string{
          {"John", "30", "NYC"},
          {"Jane", "25", "LA"},
      },
      Sortable: true,  // TerminalPrompter only
      Paginated: false,
  })
  table.Render(ctx.App.Out)
  ```

#### 18. Spinner
**Status**: ❌ Not implemented
- Loading indicator
- Animated (TerminalPrompter)
- Static text (TextPrompter)
- **API**:
  ```go
  spinner := clix.NewSpinner("Loading...")
  spinner.Start()
  defer spinner.Stop()
  ```

#### 19. Progress Bar
**Status**: ❌ Not implemented
- Progress indicator
- Percentage display
- **API**:
  ```go
  progress := clix.NewProgressBar("Uploading", 100)
  progress.Set(50) // 50%
  ```

#### 20. Badge/Tag Display
**Status**: ❌ Not implemented
- Small labeled tags/badges
- Color-coded (TerminalPrompter)
- Text-only (TextPrompter)
- **API**:
  ```go
  clix.RenderBadge(ctx.App.Out, "Active", clix.BadgeOptions{
      Color: clix.BadgeGreen, // TerminalPrompter only
      Style: clix.BadgeRounded,
  })
  
  // Multiple badges
  badges := []clix.Badge{
      {Text: "Active", Color: clix.BadgeGreen},
      {Text: "Premium", Color: clix.BadgeGold},
  }
  clix.RenderBadges(ctx.App.Out, badges, clix.BadgeOptions{
      Separator: " ",
  })
  ```

#### 21. Separator/Divider
**Status**: ❌ Not implemented
- Horizontal or vertical separator
- Styled line (TerminalPrompter)
- Simple dashes (TextPrompter)
- **API**:
  ```go
  clix.RenderSeparator(ctx.App.Out, clix.SeparatorOptions{
      Style: clix.SeparatorHorizontal,
      Char: "-", // Character to use (default: "-")
      Length: 40, // Length in characters (0 = full width)
      Label: "Section", // Optional label in center
  })
  ```

#### 22. Key-Value Pairs
**Status**: ❌ Not implemented
- Display key-value pairs in aligned columns
- **API**:
  ```go
  pairs := []clix.KeyValue{
      {Key: "Name", Value: "John Doe"},
      {Key: "Email", Value: "john@example.com"},
      {Key: "Status", Value: "Active"},
  }
  clix.RenderKeyValue(ctx.App.Out, pairs, clix.KeyValueOptions{
      Separator: ":", // Default: ":"
      Align: true,    // Align values in column (default: true)
      Width: 20,      // Key column width (default: auto)
  })
  ```

#### 23. Tree/Hierarchy Display
**Status**: ❌ Not implemented
- Display hierarchical data (e.g., file tree, org chart)
- Indented structure
- **API**:
  ```go
  tree := clix.TreeNode{
      Label: "Root",
      Children: []clix.TreeNode{
          {Label: "Child 1"},
          {Label: "Child 2", Children: []clix.TreeNode{
              {Label: "Grandchild"},
          }},
      },
  }
  clix.RenderTree(ctx.App.Out, tree, clix.TreeOptions{
      Indent: "  ", // Indentation string (default: "  ")
      Connector: "├─", // Connector characters
  })
  ```

#### 24. Alert/Notice Box
**Status**: ❌ Not implemented
- Display alert messages (success, error, warning, info)
- Bordered, color-coded boxes
- Icon support (TerminalPrompter)
- **API**:
  ```go
  clix.RenderAlert(ctx.App.Out, clix.AlertOptions{
      Type: clix.AlertSuccess, // AlertSuccess, AlertError, AlertWarning, AlertInfo
      Title: "Operation Complete",
      Message: "The file has been uploaded successfully.",
      Icon: true, // Show icon (TerminalPrompter only)
  })
  ```

#### 25. Toast Notification
**Status**: ❌ Not implemented
- Temporary status messages
- Auto-dismiss or manual dismiss
- Positioned display (TerminalPrompter)
- **API**:
  ```go
  toast := clix.NewToast(clix.ToastOptions{
      Message: "File saved",
      Type: clix.ToastSuccess,
      Duration: 3 * time.Second, // Auto-dismiss after 3s
      Position: clix.ToastTopRight, // TerminalPrompter only
  })
  toast.Show(ctx.App.Out)
  defer toast.Dismiss()
  ```

#### 26. Code Block
**Status**: ❌ Not implemented
- Display code snippets with syntax highlighting
- Monospace formatting
- Language-specific styling (TerminalPrompter)
- **API**:
  ```go
  clix.RenderCode(ctx.App.Out, clix.CodeOptions{
      Code: "func main() {\n    fmt.Println(\"Hello\")\n}",
      Language: "go", // For syntax highlighting (TerminalPrompter)
      LineNumbers: true, // Show line numbers
      Border: true, // Border around code block
  })
  ```

#### 27. Quote Block
**Status**: ❌ Not implemented
- Display quoted text
- Indented with quote marker
- Attribution support
- **API**:
  ```go
  clix.RenderQuote(ctx.App.Out, clix.QuoteOptions{
      Text: "The best way to predict the future is to invent it.",
      Attribution: "Alan Kay",
      Border: true, // Left border (TerminalPrompter: styled, TextPrompter: ASCII)
  })
  ```

#### 28. Empty State
**Status**: ❌ Not implemented
- Display when no data is available
- Icon, message, and optional action
- **API**:
  ```go
  clix.RenderEmptyState(ctx.App.Out, clix.EmptyStateOptions{
      Icon: "📭", // Emoji or symbol
      Title: "No items found",
      Message: "Try adding some items to get started.",
      Action: "Add Item", // Optional action suggestion
  })
  ```

#### 29. Fact Set (Key-Value Grid)
**Status**: ❌ Not implemented
- Display multiple key-value pairs in a grid
- Similar to Microsoft Teams FactSet
- Compact, aligned layout
- **API**:
  ```go
  facts := []clix.Fact{
      {Key: "Status", Value: "Active"},
      {Key: "Created", Value: "2024-01-15"},
      {Key: "Owner", Value: "John Doe"},
      {Key: "Region", Value: "us-east-1"},
  }
  clix.RenderFactSet(ctx.App.Out, facts, clix.FactSetOptions{
      Columns: 2, // Number of columns (default: 2)
      Separator: ":", // Key-value separator (default: ":")
  })
  ```

#### 30. Accordion/Collapsible
**Status**: ❌ Not implemented
- Collapsible sections (TerminalPrompter: interactive, TextPrompter: expanded)
- Nested content
- **API**:
  ```go
  accordion := clix.NewAccordion(clix.AccordionOptions{
      Items: []clix.AccordionItem{
          {
              Title: "Section 1",
              Content: "Content for section 1...",
              Expanded: true, // Default expanded state
          },
          {
              Title: "Section 2",
              Content: "Content for section 2...",
          },
      },
  })
  accordion.Render(ctx.App.Out) // TerminalPrompter: interactive, TextPrompter: all expanded
  ```

#### 31. Status Indicator
**Status**: ❌ Not implemented
- Visual status indicators (beyond badges)
- Dots, icons, or text
- Color-coded
- **API**:
  ```go
  clix.RenderStatus(ctx.App.Out, clix.StatusOptions{
      Status: clix.StatusOnline, // StatusOnline, StatusOffline, StatusPending, StatusError
      Label: "Server Status",
      ShowIcon: true, // Show icon/dot (TerminalPrompter: colored, TextPrompter: symbol)
  })
  ```

#### 32. Image Display
**Status**: ❌ Not implemented
- Display images (TerminalPrompter: if supported, TextPrompter: ASCII art or placeholder)
- Fallback to ASCII art or description
- **API**:
  ```go
  clix.RenderImage(ctx.App.Out, clix.ImageOptions{
      Source: "path/to/image.png", // File path or URL
      Alt: "Description of image", // Fallback text
      Width: 40, // Display width in characters
      Height: 20, // Display height in lines
  })
  ```

#### 33. Section Block
**Status**: ❌ Not implemented
- Group related content (inspired by Slack Block Kit)
- Title and fields
- **API**:
  ```go
  clix.RenderSection(ctx.App.Out, clix.SectionOptions{
      Title: "User Details",
      Fields: []clix.SectionField{
          {Label: "Name", Value: "John Doe", Short: true}, // Short: half-width
          {Label: "Email", Value: "john@example.com", Short: true},
          {Label: "Bio", Value: "Software engineer...", Short: false}, // Full width
      },
  })
  ```

### Authentication Components

Authentication components provide a beautiful, secure experience for CLI authentication flows (AuthN and AuthZ).

#### 34. Auth Code Display
**Status**: ❌ Not implemented
- Display authentication code for user to copy (like GitHub CLI)
- Prominent, easy-to-copy format
- Auto-copy to clipboard (TerminalPrompter only, optional)
- Countdown timer for expiration
- **API**:
  ```go
  clix.RenderAuthCode(ctx.App.Out, clix.AuthCodeOptions{
      Code: "ABCD-1234-EFGH-5678",
      Message: "Copy this code before opening your browser:",
      ExpiresIn: 10 * time.Minute,
      AutoCopy: true, // Copy to clipboard automatically (TerminalPrompter only)
      ShowTimer: true, // Show countdown timer
  })
  ```

#### 35. Browser Auth Flow
**Status**: ❌ Not implemented
- Guide user through browser-based authentication
- Show waiting state while browser is open
- Handle PKCE flow with local webserver
- Show success/error states
- **API**:
  ```go
  flow := clix.NewBrowserAuthFlow(clix.BrowserAuthFlowOptions{
      AuthURL: "https://example.com/oauth/authorize?code=...",
      RedirectURI: "http://localhost:8080/callback",
      Message: "Opening browser for authentication...",
      SuccessMessage: "Authentication successful!",
      ErrorMessage: "Authentication failed. Please try again.",
  })
  
  token, err := flow.Run(ctx)
  if err != nil {
      return err
  }
  // Token received, store it
  ```

#### 36. Auth Status Display
**Status**: ❌ Not implemented
- Show current authentication status
- Display active service account/token
- Show expiration times
- **API**:
  ```go
  clix.RenderAuthStatus(ctx.App.Out, clix.AuthStatusOptions{
      Authenticated: true,
      User: "user@example.com",
      ServiceAccount: "service-account@project.iam.gserviceaccount.com",
      TokenExpires: time.Now().Add(1 * time.Hour),
      Scopes: []string{"read", "write"},
  })
  ```

#### 37. Service Account Selector
**Status**: ❌ Not implemented
- Interactive selection of service account for AuthZ flow
- Shows available service accounts with descriptions
- **API**:
  ```go
  account, err := clix.SelectServiceAccount(ctx, clix.ServiceAccountSelectorOptions{
      Accounts: []clix.ServiceAccount{
          {
              Email: "sa-1@project.iam.gserviceaccount.com",
              DisplayName: "Production Service Account",
              Description: "Full access to production resources",
          },
          {
              Email: "sa-2@project.iam.gserviceaccount.com",
              DisplayName: "Development Service Account",
              Description: "Limited access to dev resources",
          },
      },
      Label: "Select service account for authorization:",
  })
  ```

## Authentication Flow Patterns

### AuthN Flow (Authentication)

The authentication flow verifies user identity:

```go
func runLogin(ctx *clix.Context) error {
    // Step 1: Generate and display auth code
    code := generateAuthCode()
    clix.RenderAuthCode(ctx.App.Out, clix.AuthCodeOptions{
        Code: code,
        Message: "Copy this code before opening your browser:",
        ExpiresIn: 10 * time.Minute,
        AutoCopy: true,
        ShowTimer: true,
    })
    
    // Step 2: Wait for user to copy code
    fmt.Fprintln(ctx.App.Out, "\nPress Enter to open browser...")
    // Wait for Enter or auto-open after delay
    
    // Step 3: Open browser with auth URL
    authURL := buildAuthURL(code)
    flow := clix.NewBrowserAuthFlow(clix.BrowserAuthFlowOptions{
        AuthURL: authURL,
        RedirectURI: "http://localhost:8080/callback",
        Message: "Opening browser for authentication...",
        SuccessMessage: "✓ Authentication successful!",
    })
    
    // Step 4: Start local server and wait for callback
    token, err := flow.Run(ctx)
    if err != nil {
        clix.RenderAlert(ctx.App.Out, clix.AlertOptions{
            Type: clix.AlertError,
            Title: "Authentication Failed",
            Message: err.Error(),
        })
        return err
    }
    
    // Step 5: Store token and show success
    storeToken(token)
    clix.RenderAlert(ctx.App.Out, clix.AlertOptions{
        Type: clix.AlertSuccess,
        Title: "Success",
        Message: "You are now authenticated.",
    })
    
    return nil
}
```

### AuthZ Flow (Authorization)

The authorization flow grants access to resources:

```go
func runAuthorize(ctx *clix.Context) error {
    // Step 1: Show current auth status
    status := getAuthStatus()
    clix.RenderAuthStatus(ctx.App.Out, clix.AuthStatusOptions{
        Authenticated: status.Authenticated,
        User: status.User,
        ServiceAccount: status.ServiceAccount,
    })
    
    // Step 2: Select service account (if multiple available)
    accounts := listServiceAccounts()
    if len(accounts) > 1 {
        account, err := clix.SelectServiceAccount(ctx, clix.ServiceAccountSelectorOptions{
            Accounts: accounts,
            Label: "Select service account:",
        })
        if err != nil {
            return err
        }
        // Use selected account
    }
    
    // Step 3: Show scopes being requested
    scopes := []string{"read", "write"}
    fmt.Fprintf(ctx.App.Out, "Requesting access to:\n")
    for _, scope := range scopes {
        fmt.Fprintf(ctx.App.Out, "  • %s\n", scope)
    }
    
    // Step 4: Run OAuth flow
    flow := clix.NewBrowserAuthFlow(clix.BrowserAuthFlowOptions{
        AuthURL: buildAuthZURL(scopes),
        RedirectURI: "http://localhost:8080/callback",
        Message: "Opening browser to grant access...",
        SuccessMessage: "✓ Access granted!",
    })
    
    token, err := flow.Run(ctx)
    if err != nil {
        return err
    }
    
    // Step 5: Store authorization token
    storeAuthZToken(token)
    
    return nil
}
```

### Complete Auth Command Example

```go
func NewAuthCommand() *clix.Command {
    auth := clix.NewGroup("auth", "Authentication and authorization")
    
    // Login (AuthN)
    login := clix.NewCommand("login")
    login.Short = "Authenticate with the service"
    login.Run = func(ctx *clix.Context) error {
        return runLogin(ctx)
    }
    
    // Logout
    logout := clix.NewCommand("logout")
    logout.Short = "Log out and clear stored credentials"
    logout.Run = func(ctx *clix.Context) error {
        clearTokens()
        clix.RenderAlert(ctx.App.Out, clix.AlertOptions{
            Type: clix.AlertSuccess,
            Message: "Logged out successfully.",
        })
        return nil
    }
    
    // Revoke (AuthZ)
    revoke := clix.NewCommand("revoke")
    revoke.Short = "Revoke authorization tokens"
    revoke.Run = func(ctx *clix.Context) error {
        revokeTokens()
        clix.RenderAlert(ctx.App.Out, clix.AlertOptions{
            Type: clix.AlertSuccess,
            Message: "Authorization revoked.",
        })
        return nil
    }
    
    // Show active service account
    active := clix.NewCommand("active")
    active.Short = "Show active service account"
    active.Run = func(ctx *clix.Context) error {
        status := getAuthStatus()
        clix.RenderAuthStatus(ctx.App.Out, clix.AuthStatusOptions{
            Authenticated: status.Authenticated,
            User: status.User,
            ServiceAccount: status.ServiceAccount,
            TokenExpires: status.TokenExpires,
            Scopes: status.Scopes,
        })
        return nil
    }
    
    auth.Children = []*clix.Command{login, logout, revoke, active}
    return auth
}
```

## Display Components API Design

### Core Principle: Write to io.Writer

Display components are **not prompts** - they render information to output. They take an `io.Writer` and write formatted content. They do **not** return user input.

### API Patterns

Display components support two API patterns:

#### Pattern 1: Function-Based (Simple)

```go
// Direct function call
clix.RenderList(w, items, clix.ListOptions{...})
clix.RenderCard(w, clix.CardOptions{...})
clix.RenderTable(w, clix.TableOptions{...})
clix.RenderKeyValue(w, pairs, clix.KeyValueOptions{...})
clix.RenderBadge(w, "Active", clix.BadgeOptions{...})
clix.RenderSeparator(w, clix.SeparatorOptions{...})
```

#### Pattern 2: Builder-Style (Complex Configurations)

```go
// Builder pattern for complex configurations
clix.NewList(items).
    SetStyle(clix.ListCard).
    SetTitle("Options").
    SetColumns(2).
    Render(w)

clix.NewTable(headers, rows).
    SetSortable(true).
    SetPaginated(true).
    SetPageSize(10).
    Render(w)
```

### Component Detection

Display components detect capabilities automatically:

```go
func RenderList(w io.Writer, items []ListItem, opts ListOptions) error {
    // Check if writer is a terminal
    if isTerminal(w) {
        return renderListTerminal(w, items, opts)
    }
    return renderListText(w, items, opts)
}
```

### TextPrompter Fallbacks

Display components must provide reasonable text-only fallbacks:

- **Lists**: 
  - Vertical: Simple bullet points `• Item`
  - Horizontal: Comma-separated `Item 1, Item 2, Item 3`
  - Card: Indented with ASCII borders
- **Cards**: ASCII box-drawing characters or simple indentation
- **Tables**: Pipe-separated `Name | Age | City` or space-aligned columns
- **Badges**: Text in brackets `[Active]` or `(Active)`
- **Progress**: ASCII bar `[████░░░░░░] 40%` or percentage text `40%`
- **Spinner**: Static text `Loading...` or `[*] Processing`
- **Key-Value**: Simple `Key: Value` format, aligned if possible
- **Separator**: Dashes `--------` or equals `========`
- **Tree**: Indented with `│`, `├─`, `└─` characters (if supported) or simple indentation

### Display Component Types

```go
// Simple function-based API (recommended for most use cases)
func RenderList(w io.Writer, items []ListItem, opts ListOptions) error
func RenderCard(w io.Writer, opts CardOptions) error
func RenderTable(w io.Writer, opts TableOptions) error
func RenderKeyValue(w io.Writer, pairs []KeyValue, opts KeyValueOptions) error
func RenderBadge(w io.Writer, text string, opts BadgeOptions) error
func RenderBadges(w io.Writer, badges []Badge, opts BadgeOptions) error
func RenderSeparator(w io.Writer, opts SeparatorOptions) error
func RenderTree(w io.Writer, root TreeNode, opts TreeOptions) error

// Builder-style API (for complex configurations)
type List struct { ... }
func NewList(items []ListItem) *List
func (l *List) SetStyle(style ListStyle) *List
func (l *List) SetTitle(title string) *List
func (l *List) Render(w io.Writer) error

type Table struct { ... }
func NewTable(opts TableOptions) *Table
func (t *Table) SetSortable(sortable bool) *Table
func (t *Table) Render(w io.Writer) error
```

### Integration with Context

Display components typically use `ctx.App.Out`:

```go
cmd.Run = func(ctx *clix.Context) error {
    // Render a list of options
    items := []clix.ListItem{
        {Label: "Option 1", Description: "First option"},
        {Label: "Option 2", Description: "Second option"},
    }
    clix.RenderList(ctx.App.Out, items, clix.ListOptions{
        Style: clix.ListVertical,
        Title: "Available Options",
    })
    return nil
}
```

### Theming Support with Lipgloss

All display components use `app.Styles` with `TextStyle` interface (compatible with lipgloss):

```go
// Component-specific style hooks in Styles struct
type Styles struct {
    // ... existing styles ...
    
    // Display component styles
    ListTitle      TextStyle // List titles
    ListItem       TextStyle // List items
    CardTitle      TextStyle // Card titles
    CardContent    TextStyle // Card content
    TableHeader    TextStyle // Table headers
    TableRow       TextStyle // Table rows
    BadgeText      TextStyle // Badge text
    AlertTitle     TextStyle // Alert titles
    AlertMessage   TextStyle // Alert messages
    CodeBlock      TextStyle // Code block content
    QuoteText      TextStyle // Quote text
    QuoteAttrib    TextStyle // Quote attribution
    StatusLabel    TextStyle // Status labels
    SectionTitle   TextStyle // Section titles
    SectionField   TextStyle // Section field labels
}

// Usage with lipgloss
app.Styles = clix.Styles{
    ListTitle: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63")),
    CardTitle: lipgloss.NewStyle().Bold(true).Border(lipgloss.RoundedBorder()),
    AlertTitle: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1")), // Red for errors
}
```

### Validation Integration

Input components use the `ext/validation` package:

```go
import "github.com/SCKelemen/clix/ext/validation"

// In PromptRequest
clix.PromptRequest{
    Label: "Email",
    Type: clix.PromptText,
    Validate: validation.Email, // Use validation extension
}

// In Argument
cmd.Arguments = []*clix.Argument{
    {
        Name: "port",
        Required: true,
        Validate: validation.Port, // Use validation extension
    },
}

// Combined validators
clix.PromptRequest{
    Label: "Password",
    Type: clix.PromptPassword,
    Validate: validation.All(
        validation.NotEmpty,
        validation.MinLength(8),
        validation.Regex(`^[a-zA-Z0-9!@#$%^&*]+$`),
    ),
}
```

### Stateful Display Components

Some display components (Spinner, ProgressBar) maintain state:

```go
// Spinner - starts/stops, writes to same position
spinner := clix.NewSpinner("Loading...")
spinner.SetWriter(ctx.App.Out) // or uses ctx.App.Out by default
spinner.Start()
defer spinner.Stop()

// ProgressBar - updates in place
progress := clix.NewProgressBar("Uploading", 100)
progress.SetWriter(ctx.App.Out)
progress.Set(50) // Updates display
progress.Set(100)
progress.Finish()
```

These components need to:
- Detect terminal capabilities
- Use ANSI codes for TerminalPrompter (clear line, move cursor)
- Use simple line-based updates for TextPrompter

## API Design

### Core Principle: Extend, Don't Replace

Components extend the existing `PromptRequest` API without breaking backward compatibility. The component type is **inferred** from the fields set, with an optional explicit `Type` field for clarity.

### Component Type Inference

Components are identified by the fields present in `PromptRequest`:

```go
// Text input (default)
PromptRequest{Label: "Name"}

// Select (has Options, no MultiSelect)
PromptRequest{Label: "Choose", Options: [...]}

// MultiSelect (has Options + MultiSelect: true)
PromptRequest{Label: "Choose", Options: [...], MultiSelect: true}

// Confirm (has Confirm: true)
PromptRequest{Label: "Continue?", Confirm: true}

// Password (has Type: PromptPassword)
PromptRequest{Label: "Password", Type: PromptPassword}

// Combobox (has Type: PromptCombobox + Options)
PromptRequest{Label: "Country", Type: PromptCombobox, Options: [...]}
```

### Explicit Type Field

For clarity and future extensibility, an optional `Type` field can be set:

```go
type PromptType int

const (
    PromptText PromptType = iota // Default, inferred from empty Options
    PromptPassword
    PromptNumber
    PromptTextarea
    PromptCombobox
    PromptSelect      // Explicit select (same as Options without MultiSelect)
    PromptMultiSelect // Explicit multi-select (same as Options + MultiSelect)
    PromptRadio
    PromptCheckbox
    PromptConfirm
    PromptFile
    PromptDate
    PromptTime
    PromptSlider
)
```

**Type inference rules:**
1. If `Type` is explicitly set, use it
2. If `Confirm == true`, type is `PromptConfirm`
3. If `Options != nil && MultiSelect == true`, type is `PromptMultiSelect`
4. If `Options != nil && MultiSelect == false`, type is `PromptSelect`
5. If `Type == PromptText` or unset, type is `PromptText`

### Component-Specific Options

Component-specific options are added via embedded structs or functional options. Two approaches:

#### Approach 1: Embedded Component Options (Recommended)

```go
type PromptRequest struct {
    // ... existing common fields ...
    Label   string
    Default string
    Validate func(string) error
    Theme   PromptTheme
    Options []SelectOption
    MultiSelect bool
    Confirm bool
    
    // Component-specific options (embedded, zero value = not used)
    Password PasswordOptions
    Number   NumberOptions
    File     FileOptions
    Date     DateOptions
    // ... etc
}

// Component-specific option structs
type PasswordOptions struct {
    Mask      bool   // Mask input (default: true)
    MaskChar  string // Character to use for masking (default: "*")
    RevealKey string // Key to toggle reveal (TerminalPrompter only, default: "Ctrl+R")
}

type NumberOptions struct {
    Min   *float64 // Optional minimum
    Max   *float64 // Optional maximum
    Step  float64  // Step increment (default: 1)
    Int   bool     // Integer-only (default: false)
}

type FileOptions struct {
    Filter    []string // File extensions to filter (e.g., [".go", ".txt"])
    Directory bool     // Pick directory instead of file (default: false)
    StartDir  string   // Starting directory (default: current dir)
}

type DateOptions struct {
    Format string    // Date format (default: "2006-01-02")
    Min    time.Time // Optional minimum date
    Max    time.Time // Optional maximum date
}
```

**Usage:**
```go
// Password
app.Prompter.Prompt(ctx, clix.PromptRequest{
    Label: "Password",
    Type: clix.PromptPassword,
    Password: clix.PasswordOptions{
        Mask: true,
        MaskChar: "•",
    },
})

// Number
app.Prompter.Prompt(ctx, clix.PromptRequest{
    Label: "Age",
    Type: clix.PromptNumber,
    Number: clix.NumberOptions{
        Min: ptr(0.0),
        Max: ptr(120.0),
        Int: true,
    },
})

// File
app.Prompter.Prompt(ctx, clix.PromptRequest{
    Label: "Config file",
    Type: clix.PromptFile,
    File: clix.FileOptions{
        Filter: []string{".yaml", ".yml"},
    },
})
```

#### Approach 2: Functional Options for Component-Specific Options

```go
// Component-specific functional options
func WithPasswordOptions(opts PasswordOptions) PromptOption
func WithNumberOptions(opts NumberOptions) PromptOption
func WithFileOptions(opts FileOptions) PromptOption
func WithDateOptions(opts DateOptions) PromptOption

// Usage
app.Prompter.Prompt(ctx,
    clix.WithLabel("Password"),
    clix.WithType(clix.PromptPassword),
    clix.WithPasswordOptions(clix.PasswordOptions{Mask: true}),
)
```

**Recommendation**: Use Approach 1 (embedded structs) for consistency with the rest of clix's struct-based API. Functional options remain available for common fields.

### Return Value Handling

The `Prompter.Prompt()` method returns `(string, error)`. For multi-value components (MultiSelect, Checkbox), return a comma-separated string:

```go
// MultiSelect returns comma-separated values
result, err := app.Prompter.Prompt(ctx, clix.PromptRequest{
    Label: "Features",
    Options: features,
    MultiSelect: true,
})
// result = "feature1,feature2,feature3"

// Helper function to parse
values := strings.Split(result, ",")
```

**Alternative**: Add a `PromptMulti()` method that returns `[]string`:

```go
// Future enhancement - not in v1
func (p Prompter) PromptMulti(ctx context.Context, opts ...PromptOption) ([]string, error)
```

For v1, use comma-separated strings for consistency with existing API.

### Component Detection in Prompter

The prompter determines component type and routes to appropriate handler:

```go
func (p TerminalPrompter) prompt(ctx context.Context, cfg *PromptConfig) (string, error) {
    // Explicit type takes precedence
    if cfg.Type != PromptText {
        return p.promptByType(ctx, cfg)
    }
    
    // Infer from fields
    if cfg.Confirm {
        return p.promptConfirm(ctx, cfg)
    }
    if len(cfg.Options) > 0 {
        if cfg.MultiSelect {
            return p.promptMultiSelect(ctx, cfg)
        }
        return p.promptSelect(ctx, cfg)
    }
    
    // Check component-specific options
    if cfg.Password.Mask {
        return p.promptPassword(ctx, cfg)
    }
    if cfg.Number.Min != nil || cfg.Number.Max != nil {
        return p.promptNumber(ctx, cfg)
    }
    if cfg.File.Filter != nil || cfg.File.Directory {
        return p.promptFile(ctx, cfg)
    }
    
    // Default to text
    return p.promptText(ctx, cfg)
}
```

### TextPrompter Fallback Strategy

### Input Components

For input components that require TTY features, TextPrompter provides line-based fallbacks:

```go
func (p TextPrompter) promptPassword(ctx context.Context, cfg *PromptConfig) (string, error) {
    // TextPrompter: read line, mask on output (don't echo input)
    // Use term package to disable echo
    return p.promptTextMasked(ctx, cfg)
}

func (p TextPrompter) promptFile(ctx context.Context, cfg *PromptConfig) (string, error) {
    // TextPrompter: simple text input with validation
    fmt.Fprintf(p.Out, "%s (enter file path): ", cfg.Label)
    path, err := p.readLine()
    if err != nil {
        return "", err
    }
    // Validate file exists, matches filter, etc.
    return path, nil
}

func (p TextPrompter) promptNumber(ctx context.Context, cfg *PromptConfig) (string, error) {
    // TextPrompter: text input with numeric validation
    value, err := p.promptText(ctx, cfg)
    if err != nil {
        return "", err
    }
    // Validate is numeric, in range, etc.
    return value, nil
}
```

### Display Components

Display components must provide ASCII/text-only fallbacks:

```go
// List Display
// TerminalPrompter: Styled, colored, with icons
// TextPrompter: Simple bullet points
//   • Item 1
//   • Item 2

// Card Display
// TerminalPrompter: Box-drawing characters, colors, padding
// TextPrompter: ASCII borders
//   +------------------+
//   | Title            |
//   +------------------+
//   | Content          |
//   +------------------+

// Table Display
// TerminalPrompter: Aligned columns, borders, sortable
// TextPrompter: Pipe-separated or space-aligned
//   Name | Age | City
//   -----|-----|-----
//   John | 30  | NYC

// Badge Display
// TerminalPrompter: Colored, rounded corners
// TextPrompter: Simple brackets
//   [Active]

// Progress Bar
// TerminalPrompter: Visual bar with colors
// TextPrompter: ASCII bar
//   [████░░░░░░] 40%

// Spinner
// TerminalPrompter: Animated spinner
// TextPrompter: Static text
//   Loading...
```

### Functional Options for Components

Extend existing functional options pattern:

```go
// Type setter
func WithType(typ PromptType) PromptOption

// Component-specific options
func WithPassword(mask bool, maskChar string) PromptOption
func WithNumber(min, max *float64, step float64, intOnly bool) PromptOption
func WithFile(filter []string, directory bool, startDir string) PromptOption
func WithDate(format string, min, max time.Time) PromptOption
func WithCombobox(allowCustom bool) PromptOption

// Usage
app.Prompter.Prompt(ctx,
    clix.WithLabel("Password"),
    clix.WithType(clix.PromptPassword),
    clix.WithPassword(true, "•"),
)
```

### Builder-Style API

Extend `PromptRequest` builder methods:

```go
func (r *PromptRequest) SetType(typ PromptType) *PromptRequest
func (r *PromptRequest) SetPassword(opts PasswordOptions) *PromptRequest
func (r *PromptRequest) SetNumber(opts NumberOptions) *PromptRequest
func (r *PromptRequest) SetFile(opts FileOptions) *PromptRequest

// Usage
clix.PromptRequest{}.
    SetLabel("Password").
    SetType(clix.PromptPassword).
    SetPassword(clix.PasswordOptions{Mask: true})
```

## Integration with Existing APIs

### Backward Compatibility

All existing `PromptRequest` usage continues to work:

```go
// Existing code - no changes needed
app.Prompter.Prompt(ctx, clix.PromptRequest{
    Label: "Name",
    Default: "unknown",
})

// New components - additive
app.Prompter.Prompt(ctx, clix.PromptRequest{
    Label: "Password",
    Type: clix.PromptPassword,
    Password: clix.PasswordOptions{Mask: true},
})
```

### Survey Integration

Components work seamlessly with Survey:

```go
s.Question("country", clix.PromptRequest{
    Label: "Country",
    Type: clix.PromptCombobox,
    Options: countries,
    // Combobox-specific options via embedded struct
}).Then("next")

s.Question("age", clix.PromptRequest{
    Label: "Age",
    Type: clix.PromptNumber,
    Number: clix.NumberOptions{
        Min: ptr(0.0),
        Max: ptr(120.0),
        Int: true,
    },
}).Then("next")
```

## Priority Implementation Order

### Input Components

#### Phase 1: High-Value, Low-Complexity
1. **Password Input** - Simple masking, high utility
2. **Number Input** - Type validation, common use case
3. **Combobox** - Autocomplete, very useful

#### Phase 2: Medium Complexity
4. **File Picker** - Useful but requires file system navigation
5. **Checkbox Group** - Explicit multi-select variant
6. **Radio Group** - Alternative to Select

#### Phase 3: Advanced Features
7. **Date/Time Pickers** - Complex parsing and validation
8. **Slider** - Visual component, requires terminal graphics

### Display Components

#### Phase 1: High-Value, Low-Complexity
1. **List Display** - Most common display need, simple fallback
2. **Key-Value Pairs** - Common for showing configuration/status
3. **Separator** - Simple but useful for organization
4. **Spinner** - Visual feedback, simple

#### Phase 2: Medium Complexity
5. **Card Display** - Useful for grouping information
6. **Progress Bar** - Visual feedback for long operations
7. **Badge Display** - Status indicators

#### Phase 3: Advanced Features
8. **Table Display** - Complex layout and interaction

## Accessibility Considerations

- **Keyboard Navigation**: All components must be fully keyboard accessible
- **Screen Readers**: Clear labels and instructions
- **Error Messages**: Accessible error display
- **Focus Management**: Clear focus indicators (TerminalPrompter)

## Theming

All components respect `PromptTheme`:
- Colors for active/inactive states
- Button styles
- Error styles
- Focus indicators

## API Examples

### Password Input

**Struct-based (primary API):**
```go
password, err := app.Prompter.Prompt(ctx, clix.PromptRequest{
    Label: "Enter password",
    Type: clix.PromptPassword,
    Password: clix.PasswordOptions{
        Mask: true,
        MaskChar: "•",
    },
    Validate: validation.MinLength(8),
})
```

**Functional options:**
```go
password, err := app.Prompter.Prompt(ctx,
    clix.WithLabel("Enter password"),
    clix.WithType(clix.PromptPassword),
    clix.WithPassword(true, "•"),
    clix.WithValidate(validation.MinLength(8)),
)
```

**Builder-style:**
```go
req := clix.PromptRequest{}.
    SetLabel("Enter password").
    SetType(clix.PromptPassword).
    SetPassword(clix.PasswordOptions{Mask: true}).
    SetValidate(validation.MinLength(8))
password, err := app.Prompter.Prompt(ctx, req)
```

### Number Input

```go
age, err := app.Prompter.Prompt(ctx, clix.PromptRequest{
    Label: "Age",
    Type: clix.PromptNumber,
    Number: clix.NumberOptions{
        Min: ptr(0.0),
        Max: ptr(120.0),
        Step: 1.0,
        Int: true,
    },
    Validate: validation.Integer,
})
```

### Combobox

```go
country, err := app.Prompter.Prompt(ctx, clix.PromptRequest{
    Label: "Country",
    Type: clix.PromptCombobox,
    Options: []clix.SelectOption{
        {Label: "United States", Value: "US"},
        {Label: "Canada", Value: "CA"},
    },
    Combobox: clix.ComboboxOptions{
        AllowCustom: true, // Allow values not in options
        Filterable: true,  // Enable filtering (TerminalPrompter only)
    },
})
```

### File Picker

```go
file, err := app.Prompter.Prompt(ctx, clix.PromptRequest{
    Label: "Select config file",
    Type: clix.PromptFile,
    File: clix.FileOptions{
        Filter: []string{".yaml", ".yml", ".json"},
        Directory: false,
        StartDir: "~/.config",
    },
})
```

### Date Picker

```go
birthDate, err := app.Prompter.Prompt(ctx, clix.PromptRequest{
    Label: "Birth date",
    Type: clix.PromptDate,
    Date: clix.DateOptions{
        Format: "2006-01-02",
        Min: time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC),
        Max: time.Now(),
    },
})
```

### MultiSelect (existing, enhanced)

```go
features, err := app.Prompter.Prompt(ctx, clix.PromptRequest{
    Label: "Select features",
    Options: []clix.SelectOption{
        {Label: "Feature A", Value: "a"},
        {Label: "Feature B", Value: "b"},
    },
    MultiSelect: true,
    // Returns: "a,b" (comma-separated)
})
```

### List Display

**Vertical list:**
```go
items := []clix.ListItem{
    {Label: "Feature A", Description: "Description of A"},
    {Label: "Feature B", Description: "Description of B"},
}
clix.RenderList(ctx.App.Out, items, clix.ListOptions{
    Style: clix.ListVertical,
    Bullet: "•",
})
```

**Horizontal list:**
```go
clix.RenderList(ctx.App.Out, items, clix.ListOptions{
    Style: clix.ListHorizontal,
    Separator: ", ",
})
```

**Card list:**
```go
clix.RenderList(ctx.App.Out, items, clix.ListOptions{
    Style: clix.ListCard,
    Title: "Available Features",
    Columns: 2, // TerminalPrompter: 2-column grid
})
```

### Card Display

```go
clix.RenderCard(ctx.App.Out, clix.CardOptions{
    Title: "User Profile",
    Content: "Name: John Doe\nEmail: john@example.com",
    Border: true,
    Padding: 1,
    Width: 50, // TerminalPrompter: fixed width
})
```

### Table Display

```go
table := clix.NewTable(clix.TableOptions{
    Headers: []string{"Name", "Age", "City"},
    Rows: [][]string{
        {"John", "30", "NYC"},
        {"Jane", "25", "LA"},
    },
    Sortable: true,  // TerminalPrompter: click headers to sort
    Paginated: false,
})
table.Render(ctx.App.Out)
```

### Key-Value Pairs

```go
pairs := []clix.KeyValue{
    {Key: "Name", Value: "John Doe"},
    {Key: "Email", Value: "john@example.com"},
    {Key: "Status", Value: "Active"},
}
clix.RenderKeyValue(ctx.App.Out, pairs, clix.KeyValueOptions{
    Separator: ":",
    Align: true, // Align values in a column
})
```

### Spinner (Display Component)

Spinner is a display component with state:

```go
spinner := clix.NewSpinner("Processing...")
spinner.Start()
defer spinner.Stop()
// ... do work ...
spinner.Stop() // or it stops on defer
```

### Progress Bar (Display Component)

```go
progress := clix.NewProgressBar("Uploading", 100) // 100 = total
progress.Set(50) // 50%
// ... more work ...
progress.Set(100) // Complete
progress.Finish()
```

### Badge Display

```go
clix.RenderBadge(ctx.App.Out, "Active", clix.BadgeOptions{
    Color: clix.BadgeGreen, // TerminalPrompter: colored, TextPrompter: ignored
    Style: clix.BadgeRounded,
})
// TerminalPrompter: [Active] (green, rounded)
// TextPrompter: [Active]
```

## Migration Path

### Backward Compatibility Guarantee

All existing `PromptRequest` usage continues to work unchanged:

```go
// Existing code - no changes needed
app.Prompter.Prompt(ctx, clix.PromptRequest{
    Label: "Name",
    Default: "unknown",
})

// Existing select - no changes needed
app.Prompter.Prompt(ctx, clix.PromptRequest{
    Label: "Choose",
    Options: options,
})

// Existing multi-select - no changes needed
app.Prompter.Prompt(ctx, clix.PromptRequest{
    Label: "Choose",
    Options: options,
    MultiSelect: true,
})

// Existing confirm - no changes needed
app.Prompter.Prompt(ctx, clix.PromptRequest{
    Label: "Continue?",
    Confirm: true,
})
```

### Type Inference Rules

1. **No breaking changes**: Existing field-based detection continues to work
2. **Explicit types are optional**: `Type` field is optional, inferred from other fields
3. **Component options are additive**: New embedded structs don't affect existing code
4. **Zero values are safe**: All component option structs have safe zero values

### Gradual Migration

Users can gradually adopt new components without changing existing code:

```go
// Phase 1: Keep existing code as-is
password, _ := app.Prompter.Prompt(ctx, clix.PromptRequest{
    Label: "Password",
    // Works as text input (no masking)
})

// Phase 2: Add component type when ready
password, _ := app.Prompter.Prompt(ctx, clix.PromptRequest{
    Label: "Password",
    Type: clix.PromptPassword, // Now gets password masking
})

// Phase 3: Add component-specific options
password, _ := app.Prompter.Prompt(ctx, clix.PromptRequest{
    Label: "Password",
    Type: clix.PromptPassword,
    Password: clix.PasswordOptions{
        Mask: true,
        MaskChar: "•",
    },
})
```

## Error Handling

### Component-Specific Errors

Components may return specific error types for better error handling:

```go
var (
    ErrComponentNotSupported = errors.New("component not supported by this prompter")
    ErrInvalidComponentType = errors.New("invalid component type")
    ErrComponentValidation  = errors.New("component validation failed")
)

// Example usage
password, err := app.Prompter.Prompt(ctx, clix.PromptRequest{
    Type: clix.PromptPassword,
    // ...
})
if errors.Is(err, clix.ErrComponentNotSupported) {
    // Fallback to text input
    password, err = app.Prompter.Prompt(ctx, clix.PromptRequest{
        Label: "Password (will be visible)",
    })
}
```

### Validation Errors

Validation errors are returned as-is from the `Validate` function:

```go
password, err := app.Prompter.Prompt(ctx, clix.PromptRequest{
    Label: "Password",
    Type: clix.PromptPassword,
    Validate: func(v string) error {
        if len(v) < 8 {
            return fmt.Errorf("password must be at least 8 characters")
        }
        return nil
    },
})
// err contains the validation error message
```

## Type Safety Considerations

### Pointer Types for Optional Values

Use pointers for optional numeric/date values to distinguish "not set" from "zero value":

```go
type NumberOptions struct {
    Min *float64 // nil = no minimum, 0.0 = minimum is 0
    Max *float64 // nil = no maximum, 0.0 = maximum is 0
    Step float64 // Always has a default (1.0)
    Int bool     // Boolean, false is meaningful default
}

// Helper function for convenience
func NumberMin(min float64) *float64 { return &min }
func NumberMax(max float64) *float64 { return &max }
```

### Zero Values

All component option structs have safe zero values:

```go
// Zero value = use defaults
PasswordOptions{} // Mask: false (but Type: PromptPassword implies Mask: true)
NumberOptions{}   // No min/max, Step: 0 (should default to 1.0)
FileOptions{}    // No filter, Directory: false
```

**Recommendation**: When `Type` is set, apply sensible defaults even if options are zero value.

## Validation Patterns

### Built-in Validators

Use existing validation extension:

```go
import "github.com/SCKelemen/clix/ext/validation"

app.Prompter.Prompt(ctx, clix.PromptRequest{
    Label: "Email",
    Type: clix.PromptText,
    Validate: validation.Email,
})

app.Prompter.Prompt(ctx, clix.PromptRequest{
    Label: "Age",
    Type: clix.PromptNumber,
    Number: clix.NumberOptions{Int: true},
    Validate: validation.Integer,
})
```

### Component-Specific Validation

Some components have built-in validation (e.g., Number validates numeric input):

```go
// Number component automatically validates numeric input
// Additional Validate function runs after type validation
age, err := app.Prompter.Prompt(ctx, clix.PromptRequest{
    Label: "Age",
    Type: clix.PromptNumber,
    Number: clix.NumberOptions{
        Min: ptr(0.0),
        Max: ptr(120.0),
    },
    // Validate runs after number validation
    Validate: func(v string) error {
        // v is already validated as a number in range
        return nil
    },
})
```

## Prompter Capability Detection

### Checking Component Support

Prompters can advertise which components they support:

```go
type Prompter interface {
    Prompt(ctx context.Context, opts ...PromptOption) (string, error)
    
    // Future: capability detection
    // SupportsComponent(typ PromptType) bool
}
```

For now, components should gracefully fall back:
- TerminalPrompter: Full support for all components
- TextPrompter: Fallback to simpler input methods

## API Completeness Checklist

### For Each Input Component

- [ ] Struct-based API (embedded options struct)
- [ ] Functional options (`With*` functions)
- [ ] Builder-style methods (`Set*` methods)
- [ ] Type inference (works without explicit `Type`)
- [ ] Explicit type support (`Type` field)
- [ ] TextPrompter fallback
- [ ] TerminalPrompter enhancement
- [ ] Validation integration
- [ ] Error handling
- [ ] Theming support
- [ ] Documentation with examples

### For Each Display Component

- [ ] Function-based API (`Render*` functions)
- [ ] Builder-style API (`New*` constructors with `Render()` method)
- [ ] Options struct for configuration
- [ ] TextPrompter fallback (ASCII/text-only)
- [ ] TerminalPrompter enhancement (styled, colored)
- [ ] Lipgloss styling hooks (via `app.Styles` with `TextStyle` interface)
- [ ] Width/formatting options
- [ ] Documentation with examples

### For Each Input Component

- [ ] Validation integration (uses `ext/validation` package)
- [ ] Lipgloss styling hooks (via `app.Styles` and prompt themes)

## Future Considerations

- **Component Composition**: Group multiple components in a single prompt
- **Custom Components**: Allow users to define custom component types
- **Component Library**: Pre-built component sets (forms, wizards)
- **Animation System**: Smooth transitions (TerminalPrompter only)
- **Capability Detection**: `Prompter.SupportsComponent(PromptType) bool`
- **Multi-Value Return**: `PromptMulti()` method returning `[]string`
- **Component Events**: Lifecycle hooks (onFocus, onBlur, onChange)

