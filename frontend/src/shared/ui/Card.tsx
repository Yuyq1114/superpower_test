import type { ReactNode } from "react";
import styles from "./Card.module.css";

export function Card(props: { title: string; action?: ReactNode; children: ReactNode }) {
  const { title, action, children } = props;

  return (
    <section className={styles.card}>
      <header className={styles.header}>
        <h2 className={styles.title}>{title}</h2>
        {action ?? null}
      </header>
      <div className={styles.body}>{children}</div>
    </section>
  );
}
