import 'package:flutter/material.dart';
import '../features/devices/devices_page.dart';
import '../features/clipboard/clipboard_page.dart';
import '../features/notes/notes_page.dart';
import '../features/transfers/transfers_page.dart';
import '../features/settings/settings_page.dart';

class ShellScreen extends StatefulWidget {
  const ShellScreen({super.key});

  @override
  State<ShellScreen> createState() => _ShellScreenState();
}

class _ShellScreenState extends State<ShellScreen> {
  int selectedIndex = 0;
  void changePage(int index) {
    setState(() {
      selectedIndex = index;
    });
  }

  final pages = [
    const DevicesPage(),
    const ClipboardPage(),
    const NotesPage(),
    const TransfersPage(),
    const SettingsPage(),
  ];

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Row(
        children: [
          Container(
            width: 220,
            color: Colors.grey.shade900,
            child: Material(
              color: Colors.transparent,
              child: Column(
                children: [
                  const SizedBox(height: 24),

                  const Text(
                    "SYNCSPACE",
                    style: TextStyle(color: Colors.white, fontSize: 20),
                  ),

                  const SizedBox(height: 24),

                  ListTile(
                    title: const Text(
                      "Devices",
                      style: TextStyle(color: Colors.white),
                    ),
                    onTap: () => changePage(0),
                  ),

                  ListTile(
                    title: const Text(
                      "Clipboard",
                      style: TextStyle(color: Colors.white),
                    ),
                    onTap: () => changePage(1),
                  ),

                  ListTile(
                    title: const Text(
                      "Notes",
                      style: TextStyle(color: Colors.white),
                    ),
                    onTap: () => changePage(2),
                  ),

                  ListTile(
                    title: const Text(
                      "Transfers",
                      style: TextStyle(color: Colors.white),
                    ),
                    onTap: () => changePage(3),
                  ),

                  ListTile(
                    title: const Text(
                      "Settings",
                      style: TextStyle(color: Colors.white),
                    ),
                    onTap: () => changePage(4),
                  ),
                ],
              ),
            ),
          ),
          Expanded(
            child: Container(
              color: Colors.grey.shade800,
              child: pages[selectedIndex],
            ),
          ),
        ],
      ),
    );
  }
}
