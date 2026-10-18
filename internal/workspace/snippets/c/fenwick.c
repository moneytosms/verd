// Fenwick tree (BIT), 0-indexed: fen_add(i, v) is a point update, fen_sum(l, r) is the sum of [l, r); O(log n)
#define FEN_N 200005
int fen_n;
long long fen_t[FEN_N + 1];
void fen_add(int i, long long v) {
    for (i++; i <= fen_n; i += i & -i) fen_t[i] += v;
}
long long fen_pre(int i) {
    long long s = 0;
    for (; i > 0; i -= i & -i) s += fen_t[i];
    return s;
}
long long fen_sum(int l, int r) { return fen_pre(r) - fen_pre(l); }
