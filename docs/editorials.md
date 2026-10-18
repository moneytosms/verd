# Editorials

Press `H` in the Problem view (or in the contest modal, for the selected problem) to read the problem's Codeforces editorial. From a shell or Neovim: `verd editorial 1900/A`.

- The text before the first hint is shown right away. **Hints, the solution and code stay closed**; `enter` opens the next one, strictly in order, so you cannot jump to the solution by accident. `o` opens the blog post in the browser, `j`/`k` scroll.
- Announcements, video tutorials and discussion streams of the contest are listed under "More materials" as links only.
- Only English editorials are shown. A contest with only a Russian one, or with none, says so.
- Codeforces problems only. CSES tasks have no editorials.
- While a [contest session](./contests.md) runs on a contest, its editorial is hidden.

On the command line, spoilers are withheld unless you ask: `verd editorial --reveal 2 1900/A` opens the first two, `--all` opens everything. `verd editorial` takes `contest/index` (`1900/A`, `1900A`) or a Solution path inside the Workspace. Exit status is 1 when there is nothing to show.

## Where it comes from

The link is the **Tutorial** entry of the "Contest materials" box on the problem page (verd reads it on the first look at a contest). The text is the blog post behind it, fetched through the Codeforces API (`blogEntry.view`), which, unlike the blog pages, is not behind the Cloudflare challenge. Both are cached in `verd.db`, so a second look works offline. A contest with no Tutorial is looked up again after six hours, since editorials are often published later.

The post covers the whole round, so verd shows the part that starts at the paragraph beginning with the problem's own link (`1900A - Name`) and ends at the next problem's. A post that is not laid out that way is shown whole, with a note.
