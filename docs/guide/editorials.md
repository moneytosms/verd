# Editorials

Stuck? Read the editorial, one hint at a time.

```sh
verd editorial 1900/A              # the idea, hints still locked
verd editorial --reveal 1 1900/A   # plus the first hint
verd editorial --all 1900/A        # everything
```

In the TUI press `H` in the Problem view (or in the contest modal for the selected problem).

## How it behaves

- The text before the first hint shows at once. **Hints, the solution and code stay closed**, and `enter` opens the next one, strictly in order. There is no way to jump to the solution by accident.
- `j`/`k` scroll, `o` opens the blog post in your browser, `q` closes.
- The contest's announcement, video tutorial and discussion stream appear under **More materials** as links.
- Only English editorials are shown. A contest with a Russian-only editorial, or none, says so.
- Only Codeforces problems have editorials (CSES does not).
- It is hidden while a [contest session](./contests.md) runs on that contest.

## Where it comes from

The link is the **Tutorial** entry of the "Contest materials" box on the problem page. The text is the blog post behind it, fetched through the Codeforces API (the blog pages themselves are behind Cloudflare). Both are cached, so a second look works offline, and a contest without an editorial is checked again after six hours.

The post covers the whole round, so verd shows the part that starts at the paragraph beginning with your problem's link (`1900A - Name`) up to the next problem's. A post laid out differently is shown whole, with a note.

```text
$ verd editorial 4/A
no editorial is published for 4A
$ echo $?
1
```

More: [Editorials](../editorials.md). Next: [Snippets and Templates](./snippets.md).
