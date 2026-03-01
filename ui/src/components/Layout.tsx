import type { ReactNode } from "react";

type Page = "landing" | "console" | "dashboard" | "docs";

interface LayoutProps {
  currentPage: Page;
  onNavigate: (page: Page) => void;
  children: ReactNode;
}

const navItems: { page: Page; label: string }[] = [
  { page: "console", label: "Console" },
  { page: "dashboard", label: "Dashboard" },
  { page: "docs", label: "Docs" },
];

export function Layout({ currentPage, onNavigate, children }: LayoutProps) {
  return (
    <div className="layout">
      <header className="header">
        <a
          className="header-logo"
          href="#"
          onClick={(e) => {
            e.preventDefault();
            onNavigate("landing");
          }}
        >
          Evidra
        </a>
        <nav className="header-nav">
          {navItems.map(({ page, label }) => (
            <a
              key={page}
              href={`#${page}`}
              className={`nav-link${currentPage === page ? " nav-link--active" : ""}`}
              onClick={(e) => {
                e.preventDefault();
                onNavigate(page);
              }}
            >
              {label}
            </a>
          ))}
          <a
            href="https://github.com/vitas/evidra"
            className="nav-link"
            target="_blank"
            rel="noopener noreferrer"
          >
            GitHub
          </a>
        </nav>
      </header>
      <main className="main">{children}</main>
    </div>
  );
}
