#!/bin/sh
# Not a .tmpl file, so trafaret copies it byte for byte: the {{ }} below is
# gh's own Go-template syntax and reaches the repo untouched.
gh pr list --author @me --json number,title --template '{{range .}}#{{.number}} {{.title}}{{"\n"}}{{end}}'
