import { Component } from '@angular/core';
import packageInfo from '../../../../../../package.json';

@Component({
    selector: 'rdio-scanner-admin-dashboard-page',
    styleUrls: ['./admin-dashboard.component.scss'],
    templateUrl: './admin-dashboard.component.html',
    standalone: false,
})
export class RdioScannerAdminDashboardPageComponent {
    version = packageInfo.version;

    constructor() {}
}
