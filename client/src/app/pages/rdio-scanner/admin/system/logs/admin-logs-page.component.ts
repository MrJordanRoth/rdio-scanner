import { AfterViewInit, Component, ViewChild } from '@angular/core';
import { RdioScannerAdminLogsComponent } from '../../../../../components/rdio-scanner/admin/logs/logs.component';

@Component({
    selector: 'rdio-scanner-admin-logs-page',
    styleUrls: ['./admin-logs-page.component.scss'],
    templateUrl: './admin-logs-page.component.html',
    standalone: false,
})
export class RdioScannerAdminLogsPageComponent implements AfterViewInit {
    @ViewChild(RdioScannerAdminLogsComponent) private logsComponent: RdioScannerAdminLogsComponent | undefined;

    async ngAfterViewInit(): Promise<void> {
        await this.logsComponent?.reload();
    }
}
