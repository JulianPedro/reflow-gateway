"use client";

import { DashboardLayout } from "@/components/dashboard-layout";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { useTheme } from "@/lib/theme";
import { useAuth } from "@/lib/auth";
import {
  Sun,
  Moon,
  Monitor,
  User,
  Bell,
  Palette,
  Info,
  Zap,
} from "lucide-react";

export default function SettingsPage() {
  const { theme, setTheme } = useTheme();
  const { user } = useAuth();

  const themeOptions = [
    { value: "light", label: "Light", icon: Sun, gradient: "from-amber-500 to-orange-500" },
    { value: "dark", label: "Dark", icon: Moon, gradient: "from-violet-500 to-purple-500" },
    { value: "system", label: "System", icon: Monitor, gradient: "from-cyan-500 to-blue-500" },
  ];

  return (
    <DashboardLayout
      title="Settings"
      description="Manage your account and application preferences"
    >
      <div className="grid gap-6 lg:grid-cols-2">
        {/* Account */}
        <Card className="overflow-hidden border-border/30 bg-card/30 backdrop-blur-xl">
          <div className="absolute left-0 top-0 h-full w-1.5 bg-gradient-to-b from-pink-500 to-rose-500" />
          <CardHeader className="pl-8">
            <CardTitle className="flex items-center gap-3 text-lg">
              <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-gradient-to-br from-pink-500 to-rose-500 text-white">
                <User className="h-5 w-5" />
              </div>
              Account
            </CardTitle>
            <CardDescription>
              Your account information
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4 pl-8">
            <div className="flex items-center justify-between rounded-xl bg-gradient-to-r from-pink-500/5 to-rose-500/5 border border-border/30 p-4">
              <div>
                <Label className="text-sm font-medium">Email</Label>
                <p className="text-sm text-muted-foreground">{user?.email}</p>
              </div>
            </div>
            <div className="flex items-center justify-between rounded-xl bg-gradient-to-r from-pink-500/5 to-rose-500/5 border border-border/30 p-4">
              <div>
                <Label className="text-sm font-medium">Role</Label>
                <p className="text-sm text-muted-foreground capitalize">{user?.role || 'user'}</p>
              </div>
            </div>
            <div className="flex items-center justify-between rounded-xl bg-gradient-to-r from-pink-500/5 to-rose-500/5 border border-border/30 p-4">
              <div>
                <Label className="text-sm font-medium">Account ID</Label>
                <p className="font-mono text-sm text-muted-foreground">{user?.id}</p>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Appearance */}
        <Card className="overflow-hidden border-border/30 bg-card/30 backdrop-blur-xl">
          <div className="absolute left-0 top-0 h-full w-1.5 bg-gradient-to-b from-violet-500 to-purple-500" />
          <CardHeader className="pl-8">
            <CardTitle className="flex items-center gap-3 text-lg">
              <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-gradient-to-br from-violet-500 to-purple-500 text-white">
                <Palette className="h-5 w-5" />
              </div>
              Appearance
            </CardTitle>
            <CardDescription>
              Customize the look and feel of the application
            </CardDescription>
          </CardHeader>
          <CardContent className="pl-8">
            <div className="space-y-4">
              <Label className="text-sm font-medium">Theme</Label>
              <div className="grid grid-cols-3 gap-3">
                {themeOptions.map((option) => (
                  <button
                    key={option.value}
                    onClick={() => setTheme(option.value as "light" | "dark" | "system")}
                    className={`relative flex flex-col items-center gap-3 rounded-xl border-2 p-5 transition-all duration-300 ${theme === option.value
                        ? "border-transparent bg-gradient-to-br " + option.gradient + " text-white shadow-lg"
                        : "border-border/30 hover:border-border/50 hover:bg-accent/20"
                      }`}
                  >
                    <option.icon className="h-6 w-6" />
                    <span className="text-sm font-medium">{option.label}</span>
                    {theme === option.value && (
                      <div className="absolute -right-1 -top-1 h-4 w-4 rounded-full bg-white shadow-lg flex items-center justify-center">
                        <div className={`h-2 w-2 rounded-full bg-gradient-to-br ${option.gradient}`} />
                      </div>
                    )}
                  </button>
                ))}
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Notifications */}
        <Card className="overflow-hidden border-border/30 bg-card/30 backdrop-blur-xl">
          <div className="absolute left-0 top-0 h-full w-1.5 bg-gradient-to-b from-amber-500 to-orange-500" />
          <CardHeader className="pl-8">
            <CardTitle className="flex items-center gap-3 text-lg">
              <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-gradient-to-br from-amber-500 to-orange-500 text-white">
                <Bell className="h-5 w-5" />
              </div>
              Notifications
            </CardTitle>
            <CardDescription>
              Configure notification preferences
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4 pl-8">
            <div className="flex items-center justify-between rounded-xl bg-gradient-to-r from-amber-500/5 to-orange-500/5 border border-border/30 p-4">
              <div>
                <Label className="text-sm font-medium">Error Alerts</Label>
                <p className="text-sm text-muted-foreground">
                  Get notified when requests fail
                </p>
              </div>
              <Switch defaultChecked />
            </div>
            <div className="flex items-center justify-between rounded-xl bg-gradient-to-r from-amber-500/5 to-orange-500/5 border border-border/30 p-4">
              <div>
                <Label className="text-sm font-medium">Target Status</Label>
                <p className="text-sm text-muted-foreground">
                  Notify when targets go offline
                </p>
              </div>
              <Switch defaultChecked />
            </div>
          </CardContent>
        </Card>

        {/* About */}
        <Card className="overflow-hidden border-border/30 bg-card/30 backdrop-blur-xl">
          <div className="absolute left-0 top-0 h-full w-1.5 bg-gradient-to-b from-cyan-500 to-blue-500" />
          <CardHeader className="pl-8">
            <CardTitle className="flex items-center gap-3 text-lg">
              <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-gradient-to-br from-cyan-500 to-blue-500 text-white">
                <Info className="h-5 w-5" />
              </div>
              About
            </CardTitle>
            <CardDescription>
              Application information
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4 pl-8">
            <div className="flex items-center justify-between rounded-xl bg-gradient-to-r from-cyan-500/5 to-blue-500/5 border border-border/30 p-4">
              <div>
                <Label className="text-sm font-medium">Version</Label>
                <p className="text-sm text-muted-foreground">1.0.0</p>
              </div>
            </div>
            <div className="flex items-center gap-4 rounded-xl bg-gradient-to-r from-violet-500/10 via-fuchsia-500/10 to-cyan-500/10 border border-border/30 p-4">
              <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-violet-500 text-white">
                <Zap className="h-6 w-6" />
              </div>
              <div>
                <Label className="text-sm font-medium">Reflow Gateway</Label>
                <p className="text-sm text-muted-foreground">
                  MCP Gateway for aggregating and proxying MCP servers
                </p>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </DashboardLayout>
  );
}
