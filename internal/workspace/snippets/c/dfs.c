// iterative DFS from s over the graphinput arrays (no recursion depth limit): stores the visit order, returns how many
int dfs(int n, int s, int *order) {
    static int st[MAXM + 1];
    static char seen[MAXN];
    for (int i = 0; i < n; i++) seen[i] = 0;
    int top = 0, cnt = 0;
    st[top++] = s;
    while (top > 0) {
        int u = st[--top];
        if (seen[u]) continue;
        seen[u] = 1;
        order[cnt++] = u;
        for (int e = head[u]; e >= 0; e = nxt[e])
            if (!seen[to[e]]) st[top++] = to[e];
    }
    return cnt;
}
