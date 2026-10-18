# iterative DFS from s (no recursion depth limit): vertices in visit order
def dfs(g, s):
    order, seen, st = [], [False] * len(g), [s]
    while st:
        u = st.pop()
        if seen[u]:
            continue
        seen[u] = True
        order.append(u)
        for v in g[u]:
            if not seen[v]:
                st.append(v)
    return order
