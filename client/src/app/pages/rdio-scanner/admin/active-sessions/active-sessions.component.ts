import { Component, OnInit } from '@angular/core';
import { MatSnackBar } from '@angular/material/snack-bar';
import { AdminActiveSession, UserManagementService } from '../user-management/user-management.service';

@Component({
    selector: 'rdio-scanner-admin-active-sessions-page',
    styleUrls: ['./active-sessions.component.scss'],
    templateUrl: './active-sessions.component.html',
    standalone: false,
})
export class RdioScannerAdminActiveSessionsPageComponent implements OnInit {
    displayedColumns: string[] = ['username', 'ipAddress', 'connectionDuration', 'action'];
    sessions: AdminActiveSession[] = [];

    constructor(
        private userManagementService: UserManagementService,
        private matSnackBar: MatSnackBar,
    ) {}

    async ngOnInit(): Promise<void> {
        await this.reload();
    }

    async reload(): Promise<void> {
        this.sessions = await this.userManagementService.listActiveSessions();
    }

    async kick(session: AdminActiveSession): Promise<void> {
        if (!session.userId) {
            return;
        }

        try {
            await this.userManagementService.kickUser(session.userId);
            this.matSnackBar.open('User kicked.', 'Close', { duration: 4000 });
            await this.reload();
        } catch {
            this.matSnackBar.open('Unable to kick user.', 'Close', { duration: 5000 });
        }
    }

    formatDuration(seconds?: number): string {
        if (typeof seconds !== 'number' || !Number.isFinite(seconds) || seconds <= 0) {
            return '0m 0s';
        }

        const total = Math.floor(seconds);
        const hours = Math.floor(total / 3600);
        const minutes = Math.floor((total % 3600) / 60);
        const remainingSeconds = total % 60;

        if (hours > 0) {
            return `${hours}h ${minutes}m ${remainingSeconds}s`;
        }

        return `${minutes}m ${remainingSeconds}s`;
    }
}
