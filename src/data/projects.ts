export interface RepoLanguage {
  name: string;
  color: string;
}

export interface Project {
  name: string;
  description: string;
  language: RepoLanguage;
  stars?: number;
}

export interface Tool {
  name: string;
  description: string;
  language: RepoLanguage;
  topics?: string[];
  license?: string;
  updated?: string;
}

export const GITHUB_USERNAME = "lazzerex";

const RUST: RepoLanguage = { name: "Rust", color: "#dea584" };
const TYPESCRIPT: RepoLanguage = { name: "TypeScript", color: "#3178c6" };
const GO: RepoLanguage = { name: "Go", color: "#00add8" };
const CSHARP: RepoLanguage = { name: "C#", color: "#178600" };
const PHP: RepoLanguage = { name: "PHP", color: "#4f5d95" };
const SVELTE: RepoLanguage = { name: "Svelte", color: "#ff3e00" };
const C: RepoLanguage = { name: "C", color: "#555555" };

export const projects: Project[] = [
  {
    name: "rustrial-os",
    description: "A hobby Rust-based operating system.",
    language: RUST,
    stars: 5
  },
  {
    name: "WordRush",
    description: "A sleek and interactive typing test web application made with Next.js.",
    language: TYPESCRIPT,
    stars: 3
  },
  {
    name: "konbi",
    description: "A minimal web application for file and notes sharing built with Go and React.",
    language: GO
  },
  {
    name: "aegis",
    description: "A high-performance network proxy built with Rust and Golang.",
    language: RUST
  },
  {
    name: "leetarena",
    description: "A LeetCode trading card game.",
    language: SVELTE
  },
  {
    name: "android-homelab",
    description: "Turning an old OPPO A37f into a homelab server.",
    language: GO
  }
];

export const tools: Tool[] = [
  {
    name: "pixel-forge",
    description:
      "High-performance image converter supporting batch processing, multiple formats, and resizing. Including Rust CLI and Windows GUI.",
    language: CSHARP
  },
  {
    name: "customer-feedback-plugin",
    description: "A WordPress plugin for collecting customers' feedback.",
    language: PHP
  },
  {
    name: "gitrpg",
    description: "Transform your GitHub activity into RPG stats and characters.",
    language: GO,
    updated: "last month"
  },
  {
    name: "xpfetch",
    description: "A fastfetch-like tool for fetching Windows XP/NT system information.",
    language: C,
    topics: ["fetch", "terminal", "command-line", "windows-xp"],
    license: "MIT",
    updated: "Apr 29"
  },
  {
    name: "takt",
    description: "A safe and minimal auto clicker app for Windows.",
    language: GO,
    topics: ["golang", "auto-clicker"],
    license: "MIT",
    updated: "Feb 10"
  }
];
