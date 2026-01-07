# How to update README with ghx screenshots

This documents the exact commands used to generate screenshots and update the README.

Prerequisites
- gh and ghx available in PATH (build ghx with `go install ./cmd/ghx`)
- termshot installed
- Repos exist at:
  - ghx repo: ~/git/asynkron/Asynkron.GitHubCliX
  - target repo to render: ~/git/asynkron/Asynkron.JsEngine

Exact command sequence

```bash
set -euo pipefail
ROOT="$HOME/git/asynkron/Asynkron.GitHubCliX"
IMGDIR="$ROOT/assets/images"
TS="$(date -u +%Y-%m-%dT%H:%M:%S.%3NZ)"

# 1) Ensure output dir and ghx binary
mkdir -p "$IMGDIR"
cd "$ROOT" && go install ./cmd/ghx

# 2) Generate screenshots in Asynkron.JsEngine
JSE="$HOME/git/asynkron/Asynkron.JsEngine"
cd "$JSE"

termshot -- ghx issue tree
cp -f out.png "$IMGDIR/ghx-issue-tree.png"

termshot -- ghx issue tree --open --closed
cp -f out.png "$IMGDIR/ghx-issue-tree-open-closed.png"

termshot -- ghx issue tree -root 399
cp -f out.png "$IMGDIR/ghx-issue-tree-root-399.png"

termshot -- ghx issue tree -root "AST delegation"
cp -f out.png "$IMGDIR/ghx-issue-tree-root-ast-delegation.png"

# 3) Append image section to README
cd "$ROOT"
cat >> README.md <<EOT

## Screenshots ($TS)

Command: ghx issue tree

![ghx issue tree](assets/images/ghx-issue-tree.png)

Command: ghx issue tree --open --closed

![ghx issue tree --open --closed](assets/images/ghx-issue-tree-open-closed.png)

Command: ghx issue tree -root 399

![ghx issue tree -root 399](assets/images/ghx-issue-tree-root-399.png)

Command: ghx issue tree -root "AST delegation"

![ghx issue tree -root AST delegation](assets/images/ghx-issue-tree-root-ast-delegation.png)
EOT

# 4) Commit and push
git add README.md assets/images/*
git commit -m "Add screenshots of ghx issue tree ($TS)"
git push
```

Removing text-based examples (optional cleanup)

```bash
cd "$HOME/git/asynkron/Asynkron.GitHubCliX"
perl -0777 -pe 's/\n## Examples \(.*?\)\n(?:.|\n)*?(?=\n## |\z)//g' -i README.md

git add README.md
git commit -m "README: remove text-based examples; keep screenshots only"
git push
```

<current_datetime>2026-01-07T11:26:22.947Z</current_datetime>

<reminder>
The user may have mentioned images, if so these will have been attached to this message, in the order they were mentioned
</reminder>
