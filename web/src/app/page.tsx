"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import styles from "./page.module.css";
import { generateDocumentId, getRecentDocuments, RecentDocument } from "@/lib/documents/recent";

export default function HomePage() {
  const router = useRouter();
  const [recent, setRecent] = useState<RecentDocument[]>([]);
  const [joinId, setJoinId] = useState("");

  useEffect(() => {
    // Reading localStorage during render would mismatch server-rendered
    // HTML (window/localStorage don't exist server-side), so this has
    // to happen post-mount. It's a one-time read of a browser-only API,
    // not the "derive state from props" antipattern the underlying
    // lint rule is meant to catch.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setRecent(getRecentDocuments());
  }, []);

  function handleCreate() {
    const id = generateDocumentId();
    router.push(`/documents/${id}?title=${encodeURIComponent("Untitled")}`);
  }

  function handleJoin(e: React.FormEvent) {
    e.preventDefault();
    const id = joinId.trim();
    if (!id) return;
    router.push(`/documents/${encodeURIComponent(id)}`);
  }

  return (
    <main className={styles.main}>
      <div className={styles.eyebrow}>sync-engine</div>
      <h1 className={styles.title}>Real-time collaborative documents</h1>
      <p className={styles.subtitle}>
        A CRDT-based sync engine, built from scratch. Every document below is a live,
        multi-writer replica reconciled with an RGA sequence CRDT — not a lock, not a queue.
      </p>

      <div className={styles.actions}>
        <button className={styles.primaryButton} onClick={handleCreate}>
          New document
        </button>

        <form className={styles.joinForm} onSubmit={handleJoin}>
          <input
            className={styles.joinInput}
            placeholder="Open by document ID…"
            value={joinId}
            onChange={(e) => setJoinId(e.target.value)}
            spellCheck={false}
          />
          <button className={styles.secondaryButton} type="submit">
            Open
          </button>
        </form>
      </div>

      <section className={styles.recentSection}>
        <h2 className={styles.recentHeading}>Recently opened</h2>
        {recent.length === 0 ? (
          <p className={styles.emptyState}>
            Nothing yet — create a document, or open one someone shared with you.
          </p>
        ) : (
          <ul className={styles.recentList}>
            {recent.map((doc) => (
              <li key={doc.id}>
                <a className={styles.recentItem} href={`/documents/${doc.id}`}>
                  <span className={styles.recentTitle}>{doc.title || "Untitled"}</span>
                  <span className={styles.recentId}>{doc.id}</span>
                </a>
              </li>
            ))}
          </ul>
        )}
      </section>
    </main>
  );
}