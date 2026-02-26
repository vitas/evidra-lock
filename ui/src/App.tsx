import { useState, useEffect } from "react";
import { Layout } from "./components/Layout";
import { Landing } from "./pages/Landing";
import { Console } from "./pages/Console";
import { Dashboard } from "./pages/Dashboard";
import { Docs } from "./pages/Docs";

type Page = "landing" | "console" | "dashboard" | "docs";

function hashToPage(hash: string): Page {
  const h = hash.slice(1);
  if (h === "console") return "console";
  if (h === "dashboard") return "dashboard";
  if (h === "docs") return "docs";
  return "landing";
}

export function App() {
  const [page, setPage] = useState<Page>(() => hashToPage(window.location.hash));

  useEffect(() => {
    const onHashChange = () => setPage(hashToPage(window.location.hash));
    window.addEventListener("hashchange", onHashChange);
    return () => window.removeEventListener("hashchange", onHashChange);
  }, []);

  const navigate = (p: Page) => {
    window.location.hash = p === "landing" ? "" : p;
    setPage(p);
  };

  return (
    <Layout currentPage={page} onNavigate={navigate}>
      {page === "landing" && <Landing onGetStarted={() => navigate("console")} />}
      {page === "console" && <Console onKeyCreated={() => navigate("dashboard")} />}
      {page === "dashboard" && <Dashboard />}
      {page === "docs" && <Docs />}
    </Layout>
  );
}
