async function getRunningContainers() {
    try {
        const response = await fetch('http://127.0.0.1:3030/list');
        const data = await response.json();
        return data.containers;
    } catch (error) {
        console.error('Error:', error);
        return [];
    }
}
async function displayContainers() {
    const data = await getRunningContainers();

    const runningContainers = document.querySelector('.running-containers');
    if (data.length > 0) {
        const table = document.createElement('table');
        table.className = 'containers-table';

        const headerRow = document.createElement('tr');
        const headers = ["Name", "Image", "Status", "CPU%", "Port(s)", "Uptime", "Actions"];
        headers.forEach(headerText => {
            const header = document.createElement('th');
            header.textContent = headerText;
            headerRow.appendChild(header);
        });
        table.appendChild(headerRow);

        data.forEach(container => {
            const row = document.createElement('tr');
            row.onclick = () => window.location.href = `/console?name=${container.name}`;

            const nameCell = document.createElement('td');
            nameCell.textContent = container.name;
            row.appendChild(nameCell);

            const imageCell = document.createElement('td');
            imageCell.textContent = container.image;
            row.appendChild(imageCell);

            const statusCell = document.createElement('td');
            statusCell.textContent = container.pid === -1 ? "Stopped" : "Running";
            row.appendChild(statusCell);

            const cpuCell = document.createElement('td');
            cpuCell.textContent = "N/A";
            row.appendChild(cpuCell);

            const portsCell = document.createElement('td');
            portsCell.textContent = container.ports;
            row.appendChild(portsCell);

            const uptimeCell = document.createElement('td');
            const currentTime = Math.floor(Date.now() / 1000);
            const uptimeSeconds = currentTime - container.timestamp;

            let uptimeText = '';
            const days = Math.floor(uptimeSeconds / 86400);
            const hours = Math.floor((uptimeSeconds % 86400) / 3600);
            const minutes = Math.floor((uptimeSeconds % 3600) / 60);
            const seconds = uptimeSeconds % 60;

            if (days > 0) {
                uptimeText += `${days}d `;
            }
            if (hours > 0) {
                uptimeText += `${hours}h `;
            }
            if (minutes > 0) {
                uptimeText += `${minutes}m `;
            }
            if (seconds > 0) {
                uptimeText += `${seconds}s`;
            }

            uptimeCell.textContent = uptimeText;
            row.appendChild(uptimeCell);

            const actionsCell = document.createElement('td');
            const stopButton = document.createElement('button');
            stopButton.textContent = "Stop";
            stopButton.className = "action-button";
            stopButton.onclick = (event) => {
                event.stopPropagation();
                alert(`Stopping ${container.name}`);
            };
            actionsCell.appendChild(stopButton);

            const deleteButton = document.createElement('button');
            deleteButton.textContent = "Delete";
            deleteButton.className = "action-button";
            deleteButton.onclick = (event) => {
                event.stopPropagation();
                alert(`Deleting ${container.name}`);
            };
            actionsCell.appendChild(deleteButton);

            row.appendChild(actionsCell);
            table.appendChild(row);
        });

        runningContainers.innerHTML = '';
        runningContainers.appendChild(table);
    } else {
        runningContainers.innerHTML = `
            <p class="running-text">Your running containers show up here</p>
            <p>(A container is an isolated environment for your code)</p>
        `;
    }
}

window.onload = () => displayContainers();