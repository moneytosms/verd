// iterative segment tree for sums over seg_n elements: seg_set(i, v) is a point update, seg_query(l, r) the sum of [l, r)
#define SEG_N 200005
int seg_n;
long long seg_t[2 * SEG_N];
void seg_set(int i, long long v) {
    for (seg_t[i += seg_n] = v; i > 1; i >>= 1) seg_t[i >> 1] = seg_t[i] + seg_t[i ^ 1];
}
long long seg_query(int l, int r) {
    long long s = 0;
    for (l += seg_n, r += seg_n; l < r; l >>= 1, r >>= 1) {
        if (l & 1) s += seg_t[l++];
        if (r & 1) s += seg_t[--r];
    }
    return s;
}
