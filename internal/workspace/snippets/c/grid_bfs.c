// BFS on a grid of '.' (free) and '#' (wall) from (sr, sc): grid_dist[r][c] is the steps to a cell, -1 if unreachable
#define GN 1005
int grid_dist[GN][GN], gq[GN * GN];
void grid_bfs(int n, int m, char g[][GN], int sr, int sc) {
    static const int dr[] = {-1, 1, 0, 0}, dc[] = {0, 0, -1, 1};
    for (int i = 0; i < n; i++)
        for (int j = 0; j < m; j++) grid_dist[i][j] = -1;
    int h = 0, t = 0;
    grid_dist[sr][sc] = 0;
    gq[t++] = sr * GN + sc;
    while (h < t) {
        int r = gq[h] / GN, c = gq[h] % GN;
        h++;
        for (int d = 0; d < 4; d++) {
            int nr = r + dr[d], nc = c + dc[d];
            if (nr >= 0 && nr < n && nc >= 0 && nc < m && g[nr][nc] != '#' && grid_dist[nr][nc] < 0) {
                grid_dist[nr][nc] = grid_dist[r][c] + 1;
                gq[t++] = nr * GN + nc;
            }
        }
    }
}
