// verd debug.h: debug(a, b, ...) prints "[L12] a = 5, b = [1, 2, 3]" to stderr.
// Handles ints, floats, strings, pairs, tuples, optionals, any container (nested), queues, stacks, heaps.
// debugm(grid) prints one row per line; debugg(adj) prints a graph as "u: v v v" (weighted: "v(w)").
// Needs C++20. Included only under -DLOCAL (verd adds it for local runs); on Codeforces debug() is a no-op.
#pragma once
#include <bits/stdc++.h>

namespace dbg_ {
using namespace std;

template <class T> string str(const T &x);

inline string str_i128(__int128 x) {
    if (x == 0) return "0";
    bool neg = x < 0;
    string s;
    for (unsigned __int128 u = neg ? -(unsigned __int128)x : x; u; u /= 10) s += char('0' + int(u % 10));
    if (neg) s += '-';
    reverse(s.begin(), s.end());
    return s;
}

template <class T> string seq(T first, T last, const char *open, const char *close) {
    string s = open;
    for (bool f = true; first != last; ++first, f = false) s += (f ? "" : ", ") + str(*first);
    return s + close;
}

template <class T> string str(const T &x) {
    if constexpr (is_same_v<T, bool>) return x ? "true" : "false";
    else if constexpr (is_same_v<T, char>) return string("'") + x + "'";
    else if constexpr (is_same_v<T, __int128>) return str_i128(x);
    else if constexpr (is_same_v<T, unsigned __int128>) return str_i128(x);
    else if constexpr (is_same_v<T, string_view>) return "\"" + string(x) + "\"";
    else if constexpr (is_convertible_v<T, string>) return "\"" + string(x) + "\"";
    else if constexpr (is_floating_point_v<T>) { ostringstream o; o << x; return o.str(); }
    else if constexpr (is_arithmetic_v<T>) return to_string(x);
    else if constexpr (is_same_v<T, vector<bool>::reference>) return x ? "true" : "false";
    else if constexpr (is_same_v<T, bitset<64>> || requires { x.to_string(); }) return x.to_string();
    else if constexpr (requires { x.has_value(); *x; }) return x.has_value() ? str(*x) : "none";
    else if constexpr (requires { x.first; x.second; }) return "(" + str(x.first) + ", " + str(x.second) + ")";
    else if constexpr (requires(T c) { c.top(); c.pop(); }) { // priority_queue / stack: print a copy, top first
        T c = x; string s = "["; bool f = true;
        for (; !c.empty(); c.pop(), f = false) s += (f ? "" : ", ") + str(c.top());
        return s + "]";
    } else if constexpr (requires(T c) { c.front(); c.pop(); }) { // queue: front first
        T c = x; string s = "["; bool f = true;
        for (; !c.empty(); c.pop(), f = false) s += (f ? "" : ", ") + str(c.front());
        return s + "]";
    } else if constexpr (requires { typename T::key_type; std::begin(x); }) return seq(std::begin(x), std::end(x), "{", "}");
    else if constexpr (requires { std::begin(x); std::end(x); }) return seq(std::begin(x), std::end(x), "[", "]");
    else if constexpr (requires { tuple_size<T>::value; }) {
        string s = "(";
        apply([&](const auto &...e) { size_t i = 0; ((s += (i++ ? ", " : "") + str(e)), ...); }, x);
        return s + ")";
    } else if constexpr (requires(ostream &o) { o << x; }) { ostringstream o; o << x; return o.str(); }
    else return "?";
}

// splits "a, f(x, y), v[2]" at top-level commas
inline vector<string> names(const string &s) {
    vector<string> r(1);
    int depth = 0;
    for (char c : s) {
        if (c == '(' || c == '[' || c == '{') depth++;
        if (c == ')' || c == ']' || c == '}') depth--;
        if (c == ',' && depth == 0) { r.emplace_back(); continue; }
        if (!(c == ' ' && r.back().empty())) r.back() += c;
    }
    return r;
}

template <class... T> void out(const char *ns, int line, const T &...xs) {
    vector<string> n = names(ns), v{str(xs)...};
    cerr << "[L" << line << "]";
    for (size_t i = 0; i < v.size(); i++) cerr << (i ? ", " : " ") << n[i] << " = " << v[i];
    cerr << endl;
}

// "v" or, for a (v, weight) pair, "v(w)"
template <class E> string edge(const E &e) {
    if constexpr (requires { e.first; e.second; }) return str(e.first) + "(" + str(e.second) + ")";
    else return str(e);
}

// debugm: one row per line, row index in front
template <class M> void matrix(const char *name, int line, const M &m) {
    cerr << "[L" << line << "] " << name << " =\n";
    size_t i = 0;
    for (const auto &row : m) {
        cerr << "  " << i++ << ":";
        for (const auto &x : row) {
            if constexpr (is_same_v<remove_cvref_t<decltype(x)>, char>) cerr << ' ' << x;
            else cerr << ' ' << str(x);
        }
        cerr << '\n';
    }
}

// debugg: adjacency list (vector or map of neighbour lists), one node per line
template <class G> void graph(const char *name, int line, const G &g) {
    cerr << "[L" << line << "] " << name << " =\n";
    auto show = [](const auto &u, const auto &adj) {
        cerr << "  " << str(u) << ":";
        for (const auto &e : adj) cerr << ' ' << edge(e);
        cerr << '\n';
    };
    if constexpr (requires { typename G::key_type; }) {
        for (const auto &[u, adj] : g) show(u, adj);
    } else {
        size_t u = 0;
        for (const auto &adj : g) show(u++, adj);
    }
}
} // namespace dbg_

#define debug(...) dbg_::out(#__VA_ARGS__, __LINE__, __VA_ARGS__)
#define debugm(m) dbg_::matrix(#m, __LINE__, m)
#define debugg(g) dbg_::graph(#g, __LINE__, g)
