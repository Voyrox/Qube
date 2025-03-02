const data = [
    {
        "name": "Qube-8SbtFD",
        "pid": -1,
        "directory": "/mnt/e/Github/Qube/tools/wasm-sample",
        "command": [
            "npm install && node index.js"
        ],
        "image": "Ubuntu24_NODE",
        "timestamp": 1740884055,
        "ports": "3000",
        "isolated": false,
        "volumes": [],
        "enviroment": []
    }
];

function displayContainers() {
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
            uptimeCell.textContent = "N/A";
            row.appendChild(uptimeCell);

            const actionsCell = document.createElement('td');
            const stopButton = document.createElement('button');
            stopButton.textContent = "Stop";
            stopButton.className = "action-button";
            stopButton.onclick = () => alert(`Stopping ${container.name}`);
            actionsCell.appendChild(stopButton);

            const deleteButton = document.createElement('button');
            deleteButton.textContent = "Delete";
            deleteButton.className = "action-button";
            deleteButton.onclick = () => alert(`Deleting ${container.name}`);
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

window.onload = displayContainers;