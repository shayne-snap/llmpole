Output demo/tui.gif
Set Shell "zsh"
Set FontSize 16
Set Width 1000
Set Height 600
Set Padding 20
Set Theme "Catppuccin Mocha"
Set WindowBar "Rings"
Set TypingSpeed 75ms

# Setup
Hide
Type "export PS1='$ '"
Enter
Type "clear"
Enter
Show

# Demo: TUI (default)
Type "llmpole"
Sleep 500ms
Enter
# Wait for TUI to load (hardware detect + model scoring)
Sleep 4s
# Navigate a bit
Type "j"
Sleep 300ms
Type "j"
Sleep 300ms
Type "k"
Sleep 300ms
# Quit TUI (q or esc)
Type "q"
Sleep 500ms
Sleep 2s
