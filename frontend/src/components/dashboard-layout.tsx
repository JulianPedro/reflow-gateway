"use client";

import { ReactNode } from "react";
import { Sidebar } from "./sidebar";
import { RequireAuth } from "@/lib/auth";

interface DashboardLayoutProps {
  children: ReactNode;
  title: string;
  description?: string;
  actions?: ReactNode;
}

export function DashboardLayout({
  children,
  title,
  description,
  actions,
}: DashboardLayoutProps) {
  return (
    <RequireAuth>
      <div className="min-h-screen bg-background">
        <Sidebar />

        <main className="pl-64">
          {/* Header */}
          <header className="sticky top-0 z-40 border-b border-border bg-background/95 backdrop-blur-sm">
            <div className="flex h-16 items-center justify-between px-6">
              <div>
                <h1 className="text-xl font-semibold tracking-tight">{title}</h1>
                {description && (
                  <p className="text-sm text-muted-foreground">{description}</p>
                )}
              </div>
              {actions && (
                <div className="flex items-center gap-3">
                  {actions}
                </div>
              )}
            </div>
          </header>

          {/* Content */}
          <div className="p-6">
            {children}
          </div>
        </main>
      </div>
    </RequireAuth>
  );
}
