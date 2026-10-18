// BFS on a grid of '.' (free) and '#' (wall) from (sr, sc): steps to each cell, -1 if unreachable
const int dr[] = {-1, 1, 0, 0}, dc[] = {0, 0, -1, 1};
vector<vector<int>> grid_bfs(const vector<string> &g, int sr, int sc) {
    int n = g.size(), m = g[0].size();
    vector<vector<int>> dist(n, vector<int>(m, -1));
    queue<pair<int, int>> q;
    dist[sr][sc] = 0;
    q.push({sr, sc});
    while (!q.empty()) {
        auto [r, c] = q.front();
        q.pop();
        for (int d = 0; d < 4; d++) {
            int nr = r + dr[d], nc = c + dc[d];
            if (nr >= 0 && nr < n && nc >= 0 && nc < m && g[nr][nc] != '#' && dist[nr][nc] < 0) {
                dist[nr][nc] = dist[r][c] + 1;
                q.push({nr, nc});
            }
        }
    }
    return dist;
}
