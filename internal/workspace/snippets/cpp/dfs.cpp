// iterative DFS from s (no recursion depth limit): vertices in visit order
vector<int> dfs(const vector<vector<int>> &g, int s) {
    vector<int> order, st{s};
    vector<bool> seen(g.size());
    while (!st.empty()) {
        int u = st.back();
        st.pop_back();
        if (seen[u]) continue;
        seen[u] = true;
        order.push_back(u);
        for (int v : g[u])
            if (!seen[v]) st.push_back(v);
    }
    return order;
}
